# ck - CLI para Kubernetes

**ck** = **C**risthian + **K**ubernetes

Uma ferramenta de linha de comando para facilitar o troubleshooting e operações do dia-a-dia no Kubernetes.

---

## Por que criar o ck?

O `kubectl` é poderoso, mas verboso. Para tarefas simples do dia-a-dia, precisamos digitar comandos longos e repetitivos.

O **ck** foi criado para:

* **Simplificar comandos frequentes** - menos digitação, mais produtividade
* **Formatar outputs** - informações organizadas e fáceis de ler
* **Filtrar o que importa** - mostrar apenas pods com problema, não todos
* **Unificar ferramentas** - kubectl + trivy + supervisorctl em um só lugar
* **Monitorar em tempo real** - detectar problemas e notificar automaticamente
* **Configuração flexível** - arquivo YAML, variáveis de ambiente e flags

---

## Configuração

O ck usa [Viper](https://github.com/spf13/viper) para gerenciamento de configuração com hierarquia de prioridade:

```
Flag (--namespace) > Variável de ambiente (CK_NAMESPACE) > Arquivo (~/.ck.yaml) > Default
```

### Arquivo de configuração (~/.ck.yaml)

```yaml
# Namespace padrão para todos os comandos
namespace: php-worker

# Número de linhas padrão para logs
tail: "100"

# Configuração do scan
scan:
  severity: "CRITICAL,HIGH"

# Configuração do watch
watch:
  restart_threshold: 3       # envia alerta quando restarts >= 3

# Notificação por email (Azure Communication Services)
notify:
  email:
    connection_string: "endpoint=https://...;accesskey=..."
    from: "DoNotReply@seudominio.com"
    to: "seu-email@destino.com"
```

### Variáveis de ambiente

Todas as configurações podem ser sobrescritas com variáveis prefixadas com `CK_`:

```bash
CK_NAMESPACE=kube-system ck pods
CK_SCAN_SEVERITY=CRITICAL ck scan nginx:latest
```

### Comandos de configuração

```bash
ck config          # Mostra configuração ativa (arquivo + valores)
ck config path     # Mostra o caminho do arquivo de configuração
```

---

## Comandos

| Comando | Descrição |
|---------|-----------|
| `ck version` | Mostra a versão do ck |
| `ck config` | Mostra configuração ativa |
| `ck pods` | Lista apenas pods com problema |
| `ck logs <pod>` | Mostra logs de um pod |
| `ck describe <pod>` | Detalhes resumidos de um pod com eventos |
| `ck exec <pod> -- <cmd>` | Executa comando dentro do pod |
| `ck top` | Lista pods por consumo de recursos |
| `ck workers` | Status dos workers do Supervisor |
| `ck scan <imagem>` | Scan de vulnerabilidades com Trivy |
| `ck ingress` | Lista ingresses com URLs |
| `ck nodes` | Status dos nodes com CPU/memória |
| `ck watch` | **Monitora pods em tempo real com alertas** |

### Flags globais

```bash
-n, --namespace <ns>    # Filtra por namespace (ou CK_NAMESPACE)
```

---

## ck watch — Monitoramento em tempo real

O `ck watch` monitora pods usando Kubernetes Informers (mesma tecnologia que Operators usam) e envia alertas por email quando detecta problemas.

### O que detecta

* **Restarts** - container reiniciou
* **CrashLoopBackOff** - container reiniciando em loop
* **OOMKilled** - container morto por falta de memória
* **Pod deletado** - pod foi removido do cluster

### Uso

```bash
# Monitora namespace do config (~/.ck.yaml)
ck watch

# Monitora namespace específico
ck watch -n php-worker

# Monitora TODOS os namespaces
ck watch -n ""
```

### Como funciona

O `ck watch` usa **client-go Informers** para manter uma conexão permanente com a API do Kubernetes. Diferente de polling (ficar perguntando "mudou algo?"), o Informer recebe eventos em tempo real conforme acontecem no cluster.

```
Cluster K8s → Evento (pod reiniciou) → Informer detecta → Alerta terminal + Email
```

Os alertas são enviados por email via **Azure Communication Services** quando o número de restarts atinge o threshold configurado (padrão: 3).

### Exemplo de alerta

```
🔄 CK ALERT

Evento:     RESTART
Pod:        php-worker-abc-123
Namespace:  php-worker
Restarts:   3
Motivo:     OOMKilled
Hora:       23/02/2026 16:03:54
─────────────────────────────────
✅ Email enviado!
```

---

## ck pods — Detecção via client-go

O comando `ck pods` usa **client-go** para se comunicar diretamente com a API do Kubernetes (sem depender do binário kubectl). Isso torna a detecção mais rápida e permite acesso a dados estruturados dos pods.

```bash
# Pods com problema no namespace do config
ck pods

# Pods com problema em namespace específico
ck pods -n kube-system

# Pods com problema em todos os namespaces
ck pods -n ""
```

---

## Sobre o comando `ck workers`

O comando `workers` foi criado para um caso de uso específico do meu ambiente de trabalho.

### Contexto

No sistema SUPP (Sistema Único de Processo e Protocolo), utilizamos pods PHP que rodam múltiplos workers gerenciados pelo **Supervisor** (um gerenciador de processos). Cada pod pode ter 50+ workers rodando simultaneamente, processando filas como:

* Indexação de documentos
* Envio de processos
* Sincronização com tribunais
* Processamento de relatórios
* E muitos outros...

### O problema

Verificar o status desses workers manualmente era trabalhoso:

```bash
# Antes: precisava entrar em cada pod e rodar supervisorctl
kubectl exec pod-xyz -n php-worker -- supervisorctl status
# Output gigante, difícil de ler
```

### A solução

```bash
# Agora: um comando mostra tudo
ck workers -n php-worker
```

Output:

```
=== WORKERS STATUS - NAMESPACE: php-worker ===

POD: php-worker-light-597447d7f8-wwt5l
  FATAL (45): indexacao_processo, download_processo, assistente_ia ... (+40 mais)
  RUNNING (14)
  Status: 14 OK | 45 FATAL | 0 STOPPED

POD: php-worker-heavy-8cf6d959b-d5fjp
  FATAL (51): indexacao_processo, populate_pessoa ... (+46 mais)
  RUNNING (8)
  Status: 8 OK | 51 FATAL | 0 STOPPED

==================================================
TOTAL GERAL: 22 OK | 96 FATAL | 0 STOPPED

ATENCAO: Workers em FATAL precisam de investigacao!
```

### Uso

```bash
# Todos os pods do namespace
ck workers -n php-worker

# Pod específico (mostra detalhes completos)
ck workers php-worker-light-xyz -n php-worker
```

> **Nota:** Este comando só funciona em pods que utilizam Supervisor. Em outros ambientes, ele mostrará "Erro: sem Supervisor".

---

## Scan de vulnerabilidades

Requer [Trivy](https://aquasecurity.github.io/trivy) instalado.

```bash
# Scan básico (usa severidade do config)
ck scan nginx:latest

# Apenas CRITICAL
ck scan nginx:latest -s CRITICAL

# Todas as severidades
ck scan minha-imagem:v1.0 -s CRITICAL,HIGH,MEDIUM,LOW
```

---

## Estrutura do projeto

```
ck/
├── main.go                 # Entrada do programa
├── go.mod                  # Dependências Go
├── go.sum                  # Lock das dependências
├── build.sh                # Script para gerar binários
├── README.md               # Este arquivo
├── CHANGELOG.md            # Histórico de mudanças
├── .gitignore              # Arquivos ignorados pelo Git
├── cmd/                    # Comandos da CLI
│   ├── root.go             # Comando raiz + Viper config
│   ├── version.go          # ck version
│   ├── config.go           # ck config
│   ├── pods.go             # ck pods (client-go)
│   ├── logs.go             # ck logs
│   ├── describe.go         # ck describe
│   ├── exec.go             # ck exec
│   ├── top.go              # ck top
│   ├── workers.go          # ck workers
│   ├── scan.go             # ck scan
│   └── watch.go            # ck watch (Informer + email)
├── k8s/                    # Integração com Kubernetes
│   ├── client.go           # Conexão client-go (kubeconfig/in-cluster)
│   └── watcher.go          # Informer + EventHandlers
├── notify/                 # Notificações
│   └── email.go            # Email via Azure Communication Services
├── types/                  # Structs compartilhadas
│   └── types.go            # Pod, Event, PodMetrics, etc
└── pratica/                # Exercícios de estudo
    ├── ponteiros.go
    ├── maps.go
    ├── loops.go
    ├── errors.go
    └── bug1.go a bug4.go
```

---

## Tecnologias

| Tecnologia | Uso |
|------------|-----|
| [Go](https://go.dev/) | Linguagem principal |
| [Cobra](https://github.com/spf13/cobra) | Framework CLI (comandos, flags, help) |
| [Viper](https://github.com/spf13/viper) | Configuração (arquivo, env, flags) |
| [client-go](https://github.com/kubernetes/client-go) | Comunicação direta com API K8s |
| [Azure Communication Services](https://azure.microsoft.com/en-us/products/communication-services) | Envio de email (alertas) |
| [Trivy](https://aquasecurity.github.io/trivy) | Scan de vulnerabilidades |

---

## Instalação

### Compilar do código fonte

Requer Go 1.21+

```bash
git clone https://github.com/crisz-source/ck.git
cd ck
go mod tidy
go build -o ck .
sudo mv ck /usr/local/bin/
```

### Configuração inicial

```bash
# Cria o arquivo de configuração
cat > ~/.ck.yaml << 'EOF'
namespace: default
tail: "100"
scan:
  severity: "CRITICAL,HIGH"
watch:
  restart_threshold: 3
notify:
  email:
    connection_string: ""
    from: ""
    to: ""
EOF
```

---

## Requisitos

* Go 1.21+ (para compilar)
* `kubectl` configurado e com acesso ao cluster
* `trivy` instalado (apenas para `ck scan`)
* Azure Communication Services (apenas para alertas do `ck watch`)

---

## Autor

Cristhian - 2026