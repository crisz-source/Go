package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	monitoringv1 "podwatcher-operator/api/v1"
	"podwatcher-operator/internal/notify"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodWatcherReconciler reconcilia um PodWatcher
type PodWatcherReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// ┌──────────────────────────────────────────────────────────┐
// │ RBAC: permissões que o Operator precisa no cluster       │
// │ Kubebuilder lê esses comentários e gera os YAMLs de RBAC│
// └──────────────────────────────────────────────────────────┘

// +kubebuilder:rbac:groups=monitoring.ck.io,resources=podwatchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.ck.io,resources=podwatchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// ┌──────────────────────────────────────────────────────────┐
// │ RECONCILE — O coração do Operator                         │
// │                                                            │
// │ É chamado AUTOMATICAMENTE quando:                          │
// │ - Um PodWatcher CR é criado/atualizado/deletado           │
// │ - Um Pod muda no namespace monitorado                      │
// │                                                            │
// │ Recebe: req.NamespacedName = qual PodWatcher mudou        │
// │ Retorna: ctrl.Result = quando rodar de novo               │
// │                                                            │
// │ Este é o seu detectChanges() + checkPodProblems()         │
// │ empacotado no formato que o Kubebuilder espera.           │
// └──────────────────────────────────────────────────────────┘

func (r *PodWatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. BUSCA O PODWATCHER CR
	//    "Qual PodWatcher disparou esse Reconcile?"
	//    É como o viper.GetString() do ck watch,
	//    mas lendo de um recurso K8s ao invés de arquivo.
	var watcher monitoringv1.PodWatcher
	if err := r.Get(ctx, req.NamespacedName, &watcher); err != nil {
		// Se não existe mais (foi deletado), ignora
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Reconciliando PodWatcher",
		"targetNamespace", watcher.Spec.TargetNamespace,
		"threshold", watcher.Spec.RestartThreshold)

	// 2. LISTA PODS DO NAMESPACE ALVO
	//    Mesmo que: clientset.CoreV1().Pods(ns).List()
	//    Mas usando o client do controller-runtime.
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(watcher.Spec.TargetNamespace)); err != nil {
		logger.Error(err, "Erro ao listar pods")
		return ctrl.Result{}, err
	}

	// 3. VERIFICA CADA POD
	//    Mesma lógica do detectChanges() do ck watch.
	threshold := watcher.Spec.RestartThreshold
	if threshold == 0 {
		threshold = 3
	}

	alertsSent := int32(0)

	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {

			// Detecta problemas (igual ao ck watch)
			if cs.RestartCount >= threshold {
				reason := "Unknown"
				if cs.LastTerminationState.Terminated != nil {
					reason = cs.LastTerminationState.Terminated.Reason
				}

				eventType := "RESTART"
				if reason == "OOMKilled" {
					eventType = "OOM_KILLED"
				}
				if cs.State.Waiting != nil &&
					cs.State.Waiting.Reason == "CrashLoopBackOff" {
					eventType = "CRASHLOOP"
				}

				event := notify.PodEvent{
					PodName:      pod.Name,
					Namespace:    pod.Namespace,
					EventType:    eventType,
					RestartCount: cs.RestartCount,
					Reason:       reason,
					Timestamp:    time.Now(),
				}

				logger.Info("Pod com problema detectado",
					"pod", pod.Name,
					"restarts", cs.RestartCount,
					"reason", reason)

				// Envia email
				if watcher.Spec.Email.ConnectionString != "" {
					if err := notify.SendEmail(event, watcher.Spec.Email); err != nil {
						logger.Error(err, "Erro ao enviar email")
					} else {
						alertsSent++
						logger.Info("Email enviado", "pod", pod.Name)
					}
				}
			}
		}
	}

	// 4. ATUALIZA O STATUS DO CR
	//    "Reporta o que encontrou de volta pro K8s"
	//    Agora kubectl get podwatchers mostra alertCount, active, etc.
	watcher.Status.Active = true
	watcher.Status.AlertCount += alertsSent
	watcher.Status.Message = fmt.Sprintf("Monitorando %d pods em %s",
		len(podList.Items), watcher.Spec.TargetNamespace)

	if alertsSent > 0 {
		now := metav1.Now()
		watcher.Status.LastAlertTime = &now
	}

	if err := r.Status().Update(ctx, &watcher); err != nil {
		logger.Error(err, "Erro ao atualizar status")
		return ctrl.Result{}, err
	}

	// 5. REAGENDA
	//    "Rode de novo em 30 segundos"
	//    No ck watch, o Informer fazia isso automaticamente.
	//    Aqui, pedimos pro controller re-rodar periodicamente.
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager configura o controller no Manager.
// Diz ao Manager: "observe PodWatchers e Pods"
func (r *PodWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.PodWatcher{}).
		Complete(r)
}
