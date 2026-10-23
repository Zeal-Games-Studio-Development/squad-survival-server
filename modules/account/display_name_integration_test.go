package account

// Opt-in test for a disposable Nakama/Postgres stack. The database container
// must use database nakama and user postgres; this test changes test-user wallets.
import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDisplayNameIntegration(t *testing.T) {
	baseURL := os.Getenv("NAKAMA_RENAME_TEST_URL")
	postgres := os.Getenv("NAKAMA_RENAME_TEST_POSTGRES_CONTAINER")
	if baseURL == "" || postgres == "" {
		t.Skip("requires disposable Nakama/Postgres integration stack")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	type reply struct {
		status int
		body   []byte
		err    error
	}
	request := func(method, path, token string, value interface{}) reply {
		body, err := json.Marshal(value)
		if err != nil {
			return reply{err: err}
		}
		req, err := http.NewRequest(method, baseURL+path, bytes.NewReader(body))
		if err != nil {
			return reply{err: err}
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			req.SetBasicAuth("defaultkey", "")
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := client.Do(req)
		if err != nil {
			return reply{err: err}
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		return reply{response.StatusCode, data, err}
	}
	check := func(r reply, code int) {
		t.Helper()
		if r.err != nil {
			t.Fatal(r.err)
		}
		if code == 0 {
			if r.status != 200 {
				t.Fatalf("HTTP %d: %s", r.status, r.body)
			}
			return
		}
		var failure struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(r.body, &failure); err != nil || failure.Code != code || r.status == 200 {
			t.Fatalf("expected code %d, HTTP %d: %s", code, r.status, r.body)
		}
	}
	sql := func(query string) string {
		t.Helper()
		out, err := exec.Command("docker", "exec", postgres, "psql", "-U", "postgres", "-d", "nakama", "-At", "-v", "ON_ERROR_STOP=1", "-c", query).CombinedOutput()
		if err != nil {
			t.Fatalf("SQL: %v: %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	newUser := func() (string, string) {
		t.Helper()
		r := request("POST", "/v2/account/authenticate/device?create=true", "", map[string]string{"id": fmt.Sprintf("rename-test-%d", time.Now().UnixNano())})
		check(r, 0)
		var session struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(r.body, &session); err != nil {
			t.Fatal(err)
		}
		r = request("GET", "/v2/account", session.Token, nil)
		check(r, 0)
		var account struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		}
		if err := json.Unmarshal(r.body, &account); err != nil || account.User.ID == "" {
			t.Fatalf("missing account: %s", r.body)
		}
		return session.Token, account.User.ID
	}
	rename := func(token, name string) reply {
		payload, _ := json.Marshal(ChangeDisplayNameRequest{DisplayName: name})
		return request("POST", "/v2/rpc/change_display_name", token, string(payload))
	}
	state := func(id string) string {
		return sql(fmt.Sprintf("SELECT coalesce(display_name,''), wallet->>'gem' FROM users WHERE id='%s'", id))
	}
	// Hold the account row so both RPCs read the same pre-update snapshot and
	// reach MultiUpdate before either can commit. This forces a real CAS conflict.
	lockAccount := func(id string) func() {
		t.Helper()
		command := exec.Command("docker", "exec", "-i", postgres, "psql", "-U", "postgres", "-d", "nakama", "-At", "-v", "ON_ERROR_STOP=1")
		input, err := command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		output, err := command.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		released := false
		release := func() {
			if released {
				return
			}
			released = true
			_, _ = io.WriteString(input, "COMMIT;\n")
			_ = input.Close()
			if err := command.Wait(); err != nil {
				t.Errorf("release account lock: %v", err)
			}
		}
		t.Cleanup(release)
		ready := make(chan bool, 1)
		go func() {
			scanner := bufio.NewScanner(output)
			for scanner.Scan() {
				if scanner.Text() == "rename_lock_ready" {
					ready <- true
					_, _ = io.Copy(io.Discard, output)
					return
				}
			}
			ready <- false
		}()
		_, err = fmt.Fprintf(input, "BEGIN;\nSELECT id FROM users WHERE id='%s' FOR UPDATE;\n\\echo rename_lock_ready\n", id)
		if err != nil {
			release()
			t.Fatal(err)
		}
		select {
		case ok := <-ready:
			if !ok {
				release()
				t.Fatal("account lock was not acquired")
			}
		case <-time.After(10 * time.Second):
			release()
			t.Fatal("timed out acquiring account lock")
		}
		return release
	}
	token, id := newUser()
	sql(fmt.Sprintf(`UPDATE users SET wallet='{"gem":0,"gold":1000}' WHERE id='%s'`, id))
	check(rename(token, "FirstName"), 0)
	if got := state(id); got != "FirstName|0" {
		t.Fatal(got)
	}
	check(rename(token, "  FirstName  "), 0)
	check(rename(token, "SecondName"), 9)
	if got := state(id); got != "FirstName|0" {
		t.Fatalf("insufficient funds did not roll back: %s", got)
	}
	// Insufficient-funds rollback also preserves the operation version.
	version := sql(fmt.Sprintf("SELECT version FROM storage WHERE user_id='%s' AND collection='account_operations'", id))
	check(rename(token, "SecondName"), 9)
	if got := sql(fmt.Sprintf("SELECT version FROM storage WHERE user_id='%s' AND collection='account_operations'", id)); got != version {
		t.Fatal("failed transaction changed guard")
	}
	sql(fmt.Sprintf(`UPDATE users SET wallet='{"gem":500,"gold":1000}' WHERE id='%s'`, id))
	check(rename(token, "Bad!"), 3)
	if got := state(id); got != "FirstName|500" {
		t.Fatal(got)
	}
	check(rename(token, "SecondName"), 0)
	if got := state(id); got != "SecondName|0" {
		t.Fatal(got)
	}
	if got := sql(fmt.Sprintf("SELECT count(*) FROM wallet_ledger WHERE user_id='%s' AND changeset->>'gem'='-500' AND metadata->>'event'='change_display_name' AND NOT (metadata ? 'reason')", id)); got != "1" {
		t.Fatalf("unexpected payment ledger: %s", got)
	}
	if got := sql(fmt.Sprintf("SELECT read, write FROM storage WHERE user_id='%s' AND collection='account_operations'", id)); got != "0|0" {
		t.Fatalf("guard permissions: %s", got)
	}
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprint("concurrent_same=", same), func(t *testing.T) {
			token, id := newUser()
			sql(fmt.Sprintf(`UPDATE users SET wallet='{"gem":500}' WHERE id='%s'`, id))
			release := lockAccount(id)
			start := make(chan struct{})
			results := make(chan reply, 2)
			for i := range 2 {
				name := "PlayerOne"
				if i == 1 && !same {
					name = "PlayerTwo"
				}
				go func() { <-start; results <- rename(token, name) }()
			}
			close(start)
			deadline := time.Now().Add(10 * time.Second)
			for sql("SELECT count(*) FROM pg_stat_activity WHERE datname='nakama' AND wait_event_type='Lock'") != "2" {
				if time.Now().After(deadline) {
					release()
					t.Fatal("RPCs did not reach the transaction lock")
				}
				time.Sleep(25 * time.Millisecond)
			}
			release()
			for range 2 {
				check(<-results, 0)
			}
			want := "0"
			if same {
				want = "500"
			}
			if got := sql(fmt.Sprintf("SELECT wallet->>'gem' FROM users WHERE id='%s'", id)); got != want {
				t.Fatalf("concurrent balance: got %s want %s", got, want)
			}
		})
	}
}
