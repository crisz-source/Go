package notify
package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PodEvent struct {
	PodName      string
	Namespace    string
	EventType    string
	RestartCount int32
	Reason       string
	Timestamp    time.Time
}

// EmailConfig recebe os dados do CRD (não do Viper!)
type EmailConfig struct {
	From             string
	To               string
	ConnectionString string
}

// SendEmail — mesma função do ck, mas recebe config como parâmetro
// ao invés de ler do Viper. Porque no Operator, a config vem do CRD.
func SendEmail(event PodEvent, config EmailConfig) error {
	if config.ConnectionString == "" || config.From == "" || config.To == "" {
		return fmt.Errorf("email não configurado no PodWatcher CR")
	}

	endpoint, accessKey, err := parseConnectionString(config.ConnectionString)
	if err != nil {
		return fmt.Errorf("connection string inválida: %w", err)
	}

	subject, body := formatEmail(event)

	payload := map[string]interface{}{
		"senderAddress": config.From,
		"recipients": map[string]interface{}{
			"to": []map[string]string{
				{"address": config.To},
			},
		},
		"content": map[string]string{
			"subject":   subject,
			"plainText": body,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro ao criar JSON: %w", err)
	}

	apiURL := endpoint + "/emails:send?api-version=2023-03-31"
	return sendWithHMAC(apiURL, accessKey, jsonPayload)
}

func sendWithHMAC(apiURL string, accessKey string, jsonPayload []byte) error {
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return fmt.Errorf("URL inválida: %w", err)
	}

	host := parsedURL.Host
	pathAndQuery := parsedURL.RequestURI()
	contentHash := computeContentHash(jsonPayload)
	timestamp := time.Now().UTC().Format(http.TimeFormat)

	stringToSign := fmt.Sprintf("POST\n%s\n%s;%s;%s",
		pathAndQuery, timestamp, host, contentHash)

	signature, err := computeSignature(stringToSign, accessKey)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ms-date", timestamp)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("Authorization",
		fmt.Sprintf("HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=%s", signature))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Azure API status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return base64.StdEncoding.EncodeToString(hash[:])
}

func computeSignature(stringToSign string, accessKey string) (string, error) {
	decodedKey, err := base64.StdEncoding.DecodeString(accessKey)
	if err != nil {
		return "", fmt.Errorf("erro ao decodificar access key: %w", err)
	}
	mac := hmac.New(sha256.New, decodedKey)
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func parseConnectionString(connStr string) (string, string, error) {
	var endpoint, accessKey string
	parts := strings.Split(connStr, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "endpoint=") {
			endpoint = strings.TrimPrefix(part, "endpoint=")
			endpoint = strings.TrimRight(endpoint, "/")
		} else if strings.HasPrefix(part, "accesskey=") {
			accessKey = strings.TrimPrefix(part, "accesskey=")
		}
	}
	if endpoint == "" || accessKey == "" {
		return "", "", fmt.Errorf("connection string deve ter endpoint= e accesskey=")
	}
	return endpoint, accessKey, nil
}

func formatEmail(event PodEvent) (string, string) {
	emoji := "⚠️"
	switch event.EventType {
	case "CRASHLOOP":
		emoji = "🔴"
	case "OOM_KILLED":
		emoji = "💀"
	case "RESTART":
		emoji = "🔄"
	}

	subject := fmt.Sprintf("%s PodWatcher Alert: %s - %s/%s",
		emoji, event.EventType, event.Namespace, event.PodName)

	body := fmt.Sprintf(
		"PODWATCHER ALERT\n"+
			"════════════════════════════════\n\n"+
			"Evento:     %s\n"+
			"Pod:        %s\n"+
			"Namespace:  %s\n"+
			"Restarts:   %d\n"+
			"Motivo:     %s\n"+
			"Hora:       %s\n\n"+
			"════════════════════════════════\n"+
			"Enviado por PodWatcher Operator",
		event.EventType,
		event.PodName,
		event.Namespace,
		event.RestartCount,
		event.Reason,
		event.Timestamp.Format("02/01/2006 15:04:05"),
	)

	return subject, body
}