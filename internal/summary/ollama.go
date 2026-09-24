package summary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

var Sizes = map[string]struct{}{
	"gist": {}, "executive": {}, "meeting": {},
}

func ParseSize(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "short":
		return "gist", nil
	case "medium":
		return "executive", nil
	case "long":
		return "meeting", nil
	}
	if _, ok := Sizes[s]; ok {
		return s, nil
	}
	return "", fmt.Errorf("unknown summary size %q", s)
}

const lang = "Отвечай на том же языке, что и транскрипт, если пользователь явно не просит иной язык."

func SystemInstruction(size string) string {
	switch size {
	case "gist":
		return "Ты анализируешь транскрипт. Не пересказывай дословно. Ответь структурно: " +
			"(1) один абзац — о чём речь и в каком контексте; " +
			"(2) маркированный список — главные темы или вопросы; " +
			"(3) одна строка — итоговый вывод в одном предложении; " +
			"(4) опционально одна строка — для кого это полезно, только если это следует из текста. " +
			"Если в тексте нет данных — не выдумывай; пиши «не указано в транскрипте». " +
			"Ограничь общий объём примерно 120–180 слов. " + lang
	case "executive":
		return "Составь резюме документа или встречи для руководителя: примерно до одной страницы текста. " +
			"Структура: краткий заголовок; Контекст (2–3 предложения); Ключевые выводы (5–7 маркеров); " +
			"Принятые решения; Риски и неясности (только из текста); Следующие шаги (кто/что/когда — " +
			"только если в транскрипте явно или уверенно следует). Не цитируй длинно; не добавляй фактов, " +
			"которых нет в транскрипте; там где неясно — «не указано в транскрипте». " + lang
	default:
		return "Подготовь полный доклад по стенограмме встречи: структурированный текст с подзаголовками (##). " +
			"Включи: участников, если звучат имена, иначе явно укажи, что участники не названы; темы по порядку; " +
			"суть дискуссии и позиции по блокам; промежуточные выводы по разделам; итоговые решения отдельным разделом; " +
			"список поручений (задача — ответственный — срок) только если это есть в тексте; открытые вопросы в конце. " +
			"Стиль нейтрального протокола. Не выдумывай участников и договорённости; пропуски помечай " +
			"«не указано в транскрипте». " + lang
	}
}

// transcriptRuneBudget leaves room in a 16k-token context for the system
// prompt, chat template and ~2k completion tokens. Russian is ~2–4 chars/token.
const transcriptRuneBudget = 28000

func clip(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	return string(r[:maxRunes]) + "\n…[truncated]"
}

type Client struct {
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func New(baseURL, model string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		HTTP:    &http.Client{Timeout: 15 * time.Minute},
	}
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("llama-server unreachable at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("llama-server HTTP %d", resp.StatusCode)
	}
	return nil
}

type chatReq struct {
	Messages    []chatMsg `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
}

func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(chatReq{
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream:      false,
		Temperature: 0.2,
		MaxTokens:   2048,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("llama-server request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llama-server HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed chatResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("unexpected llama-server response")
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("empty llama-server response")
	}
	return parsed.Choices[0].Message.Content, nil
}

func Summarize(ctx context.Context, c *Client, transcript, size, customPrompt string) (text string, mode string, err error) {
	t := clip(strings.TrimSpace(transcript), transcriptRuneBudget)
	if t == "" {
		return "", "", fmt.Errorf("empty transcript")
	}
	if strings.TrimSpace(customPrompt) != "" {
		user := customPrompt + "\n\n--- Транскрипт ---\n" + t + "\n--- Конец транскрипта ---"
		out, err := c.Chat(ctx, "Следуй инструкции пользователя. Не выдумывай факты. "+lang, user)
		return out, "custom", err
	}
	sys := SystemInstruction(size)
	user := "--- Транскрипт ---\n" + t + "\n--- Конец транскрипта ---\n\nСформируй результат строго по системной инструкции."
	out, err := c.Chat(ctx, sys, user)
	return out, "single", err
}
