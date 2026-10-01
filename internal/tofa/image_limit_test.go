package tofa_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestAdapterBoundsDeepSeekImageOutputWithOneNotice(t *testing.T) {
	const body = `{"model":"deepseek-ai/DeepSeek-V4.1-Flash","input":[{"role":"user","content":[{"type":"input_text","text":"What colour?"},{"type":"input_image","image_url":"data:image/png;base64,fixture","detail":"high"}]}],"stream":true}`
	app, output := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			MaxOutputTokens int `json:"max_output_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if request.MaxOutputTokens != 32768 {
			fmt.Fprint(w, "data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":null}}\n\n")
			return
		}
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
	}, func(endpoint, token string) error {
		for i := 0; i < 2; i++ {
			response := adapterRequest(t, endpoint, token, body)
			raw, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(raw), `"response.completed"`) {
				t.Errorf("image request did not complete: status=%d body=%s error=%v", response.StatusCode, raw, err)
			}
		}
		return nil
	})
	runAdapted(t, app)
	if strings.Count(output.String(), "DeepSeek image output:") != 1 || !strings.Contains(output.String(), "32768") || !strings.Contains(output.String(), "reasoning and answer") {
		t.Errorf("missing or repeated output-limit notice: %s", output.String())
	}
}

func TestAdapterPreservesImageOutputContracts(t *testing.T) {
	const image = `[{"role":"user","content":[{"type":"input_image","image_url":"https://example.invalid/fixture.png","detail":"high"}]}]`
	for _, tc := range []struct {
		name, model, input, limit string
		changed                   bool
	}{
		{"missing limit", "deepseek-ai/DeepSeek-V4.1-Flash", image, "", true},
		{"explicit higher limit", "deepseek-ai/DeepSeek-V4.1-Flash", image, "65536", false},
		{"explicit lower limit", "deepseek-ai/DeepSeek-V4.1-Flash", image, "100", false},
		{"explicit null", "deepseek-ai/DeepSeek-V4.1-Flash", image, "null", false},
		{"invalid explicit zero remains provider input", "deepseek-ai/DeepSeek-V4.1-Flash", image, "0", false},
		{"invalid explicit string remains provider input", "deepseek-ai/DeepSeek-V4.1-Flash", image, `"invalid"`, false},
		{"other image model", "moonshotai/Kimi-K3", image, "", false},
		{"other main", "zai-org/GLM-5.3-Flash", image, "", false},
		{"text mentioning image type", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"role":"user","content":[{"type":"input_text","text":"input_image"}]}]`, "", false},
		{"string input", "deepseek-ai/DeepSeek-V4.1-Flash", `"Explain input_image"`, "", false},
		{"tool output is not image input", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"type":"function_call_output","call_id":"fixture","output":"input_image"}]`, "", false},
		{"tool returned image", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"type":"function_call_output","call_id":"fixture","output":[{"type":"input_image","image_url":"data:image/png;base64,fixture"}]}]`, "", true},
		{"tool returned text", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"type":"function_call_output","call_id":"fixture","output":[{"type":"input_text","text":"input_image"}]}]`, "", false},
		{"custom tool returned image", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"type":"custom_tool_call_output","call_id":"fixture","output":[{"type":"input_image","image_url":"data:image/png;base64,fixture"}]}]`, "", true},
		{"custom tool returned text", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"type":"custom_tool_call_output","call_id":"fixture","output":[{"type":"input_text","text":"input_image"}]}]`, "", false},
		{"unrelated output field", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"type":"future_item","output":[{"type":"input_image","image_url":"fixture"}]}]`, "", false},
		{"image retained among mixed history", "deepseek-ai/DeepSeek-V4.1-Flash", `[{"role":"developer","content":"Retain instructions"},{"role":"user","content":[{"type":"input_image","file_id":"synthetic-file"}]},{"type":"function_call_output","call_id":"fixture","output":"prior result"},{"role":"user","content":"Continue"}]`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"` + tc.model + `","input":` + tc.input + `,"reasoning":{"effort":"none"},"future":9007199254740993,"tools":[{"type":"function","name":"fixture","parameters":{"type":"object"}}]`
			want := body
			if tc.limit != "" {
				body += `,"max_output_tokens":` + tc.limit
				want += `,"max_output_tokens":` + tc.limit
			} else if tc.changed {
				want += `,"max_output_tokens":32768`
			}
			body += `}`
			want += `}`
			// A bound can be reached upstream. Preserve its native incomplete
			// response rather than inventing successful completion or retrying.
			const stream = "data: {\"type\":\"response.incomplete\",\"response\":{\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"}}}\n\n"
			calls := 0
			app, output := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				raw, err := io.ReadAll(r.Body)
				if err != nil || !reflect.DeepEqual(jsonValue(t, raw), jsonValue(t, []byte(want))) {
					t.Errorf("request fields changed: %s error=%v", raw, err)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, stream)
			}, func(endpoint, token string) error {
				response := adapterRequest(t, endpoint, token, body)
				raw, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || string(raw) != stream {
					t.Errorf("provider stream changed: %s error=%v", raw, err)
				}
				return nil
			})
			runAdapted(t, app)
			if calls != 1 {
				t.Errorf("unexpected upstream request count: %d", calls)
			}
			if (strings.Count(output.String(), "DeepSeek image output:") == 1) != tc.changed {
				t.Errorf("output-limit notice does not match adjustment: %s", output.String())
			}
		})
	}
}
