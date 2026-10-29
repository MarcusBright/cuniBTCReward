package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

const sendTimeout = 5 * time.Second

// Use the standard client so sending a log never generates another logx entry.
var webhookClient = &http.Client{Timeout: sendTimeout}

type slackWriter struct {
	client  *http.Client
	url     string
	queue   chan slackMessage
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	dropped atomic.Uint64
}

type slackMessage struct {
	Text string `json:"text"`
}

func NewSlackWriter(url string) *slackWriter {
	return newSlackWriter(url, webhookClient)
}

func newSlackWriter(url string, client *http.Client) *slackWriter {
	ctx, cancel := context.WithCancel(context.Background())
	s := &slackWriter{
		client: client,
		url:    url,
		queue:  make(chan slackMessage, 128),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go s.send()
	return s
}

func (s *slackWriter) Write(p []byte) (n int, err error) {
	if s.url == "" || s.ctx.Err() != nil {
		return len(p), nil
	}
	var entry struct {
		Level   string `json:"level"`
		Server  any    `json:"server"`
		Content any    `json:"content"`
	}
	if err := json.Unmarshal(p, &entry); err != nil || entry.Level != "error" {
		return len(p), nil
	}
	msg := slackMessage{Text: fmt.Sprintf("[%v] %v", entry.Server, entry.Content)}
	select {
	case <-s.ctx.Done():
	case s.queue <- msg:
	default:
		// Slack is best effort: a full queue must never block the caller.
		s.dropped.Add(1)
	}
	return len(p), nil
}

// Dropped returns the number of error logs discarded because the queue was full.
func (s *slackWriter) Dropped() uint64 {
	return s.dropped.Load()
}

// Close cancels the in-flight request and discards pending logs. The queue is
// intentionally left open so concurrent or late writes cannot panic.
func (s *slackWriter) Close() error {
	s.cancel()
	<-s.done
	return nil
}

func (s *slackWriter) send() {
	defer close(s.done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.queue:
			if s.ctx.Err() != nil {
				return
			}
			// Do not log failures here: that would feed this same writer again.
			_ = postWebhook(s.ctx, s.client, s.url, msg)
		}
	}
}

func postWebhook(ctx context.Context, client *http.Client, url string, msg slackMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Slack responses are small. Bound draining and allow connection reuse.
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func SendTo(url, message string) {
	if url == "" || message == "" {
		return
	}
	_ = postWebhook(context.Background(), webhookClient, url, slackMessage{Text: message})
}
