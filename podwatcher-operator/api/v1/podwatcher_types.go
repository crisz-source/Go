package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodWatcherSpec define o estado DESEJADO do PodWatcher.
// É o que o usuário preenche no YAML.
// Equivale ao que você tinha no ~/.ck.yaml, mas agora
// é um recurso K8s de verdade.
type PodWatcherSpec struct {

	// Namespace que o Operator vai monitorar
	// No ck watch era: viper.GetString("namespace")
	// Agora é um campo do CRD.
	TargetNamespace string `json:"targetNamespace"`

	// Número de restarts pra disparar alerta
	// No ck watch era: viper.GetInt32("watch.restart_threshold")
	// +kubebuilder:default=3
	RestartThreshold int32 `json:"restartThreshold,omitempty"`

	// Configuração de email
	Email EmailConfig `json:"email"`
}

// EmailConfig contém as configurações de email.
// Os mesmos campos que estavam no ~/.ck.yaml
type EmailConfig struct {
	From             string `json:"from"`
	To               string `json:"to"`
	ConnectionString string `json:"connectionString"`
}

// PodWatcherStatus define o estado ATUAL (observado) do PodWatcher.
// O Operator preenche isso automaticamente.
// É onde o Operator reporta o que está acontecendo.
type PodWatcherStatus struct {

	// Está monitorando?
	Active bool `json:"active,omitempty"`

	// Último alerta enviado
	LastAlertTime *metav1.Time `json:"lastAlertTime,omitempty"`

	// Total de alertas enviados
	AlertCount int32 `json:"alertCount,omitempty"`

	// Mensagem de status
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target NS",type=string,JSONPath=`.spec.targetNamespace`
// +kubebuilder:printcolumn:name="Threshold",type=integer,JSONPath=`.spec.restartThreshold`
// +kubebuilder:printcolumn:name="Alerts",type=integer,JSONPath=`.status.alertCount`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`

// PodWatcher é o Schema do recurso podwatchers
type PodWatcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodWatcherSpec   `json:"spec,omitempty"`
	Status PodWatcherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PodWatcherList contém uma lista de PodWatcher
type PodWatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodWatcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodWatcher{}, &PodWatcherList{})
}
