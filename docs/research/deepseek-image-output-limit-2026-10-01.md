# DeepSeek image failure with omitted output limit

During #35 qualification, rc11 (`3ce3edbe0f91707af4b58aa83c3c8704684bb6c2`)
with desktop engine 0.159.2 failed twice on the user's first image question.
The input was a synthetic 32-by-32 red PNG. The desktop reported
`stream disconnected before completion: response.failed event received`.
The first saved native turn records 2.55 seconds and reasoning effort `none`.
This is an unresolved release qualification failure, not a passing image check.

## Reproduction and controls

All observations below use the installed rc11 adapter and the exact model
`deepseek-ai/DeepSeek-V4.1-Flash`. No real conversation content or credentials are
included in the captured reports. Native probes use the installed engine in a
disposable home/workspace and an authenticated local observer. They do not change
the running desktop or production launcher. Prompts request only the image's
colour and prohibit tools. Individual requests have finite deadlines and stream
size bounds; results are not general model benchmarks.

| Request | Result | Seconds |
| --- | --- | --- |
| Small image request, explicit 2,048 output-token limit, provider-default reasoning | Completed; red identified | 0.953 |
| Small image request, same limit, native `none` effort | Completed; red identified | 20.887 |
| Actual native engine image request, no output limit | `response.failed`, native user-visible error reproduced | 1.588 |
| Native request with tool declaration group removed, no output limit | Same failure; two `internal_error` events | 1.079 |
| Native request with explicit 2,048 output-token limit | Completed; red identified, eight tool definitions retained | 2.088 |
| Small image request with its output limit removed | Same failed stream and `internal_error` | 0.543 |
| Native request with image removed, no output limit | Completed stream; no provider failure | 1.587 |
| Native image request with explicit 32,768 output-token limit | Completed; red identified, eight tool definitions retained | 1.589 |

The no-image control cannot identify an absent red image. Its original observer
therefore returned failure on the colour assertion, although native completion
and the provider stream both succeeded. It is evidence for a completed text
request, not a successful image task.

The provider returns HTTP 200 followed by `response.created`,
`response.in_progress`, and `response.failed`. The failed response has status
`failed`, a null `error` and null `incomplete_details`. Subsequent error events
carry code `internal_error` and message `Failed to process the request`.

The native diagnostic feedback loop is
`python3 /private/tmp/tofa-35-native-image-probe/run.py NAME`, where NAME must be
new to preserve each report. Default behavior reproduces the failure. Setting
`TOFA_NATIVE_IMAGE_VARIANT=bounded-output` adds the tested 2,048 limit; setting
`TOFA_NATIVE_IMAGE_OUTPUT_TOKENS=32768` with that variant tests the larger limit.
`no-tools` and `no-image` variants remove only their respective input group.
These are retained local diagnostic scripts, not distributed launcher commands.

## Observer failures retained separately

The earliest small probe omitted the native image `detail: high` field and
received HTTP 422. Its first error reader truncated the JSON body and mislabelled
it as non-JSON. A later read established a JSON validation response. Adding the
observed image-detail field corrected the probe; these HTTP 422 results do not
reproduce the desktop's failed SSE response.

The first native observer assumed the failed response's `error` was an object.
Its null value interrupted forwarding after the observer recorded
`response.failed`, causing the client to report premature closure. The corrected
observer forwards the event unchanged and reproduces the exact user-visible
`response.failed event received` error. Earlier reports remain distinct.

## Conclusion and approved workaround

The demonstrated trigger is image input combined with an omitted
`max_output_tokens`. An explicit limit makes both the small request and the full
native-engine request complete. This establishes an upstream default-handling
compatibility problem; it does not establish the provider's internal root cause,
maximum supported output, or whether every image size/model has the same defect.
Tool declarations are not required to trigger it, and native `none` effort works
when an explicit limit is supplied.

The [Nebius Responses reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response)
lists `max_output_tokens` as optional and nullable. It bounds both reasoning and
visible output, without publishing a numeric default or model-specific ceiling.
Consequently the 32,768 workaround is a launcher policy, not a verified
provider maximum/default. The short successful request shows that this value is
accepted, not that a 32,768-token response was produced or quality-qualified.

The user explicitly approved the narrow 32,768-token workaround on October 1.
The shared adapter now supplies it only for this model's image-bearing requests
when the field is absent, announces it once, preserves explicit limits (including
null) and other requests, and retains native incomplete/error semantics. A public
HTTP regression reproduced the missing-limit failed stream before the change and
passed afterward. Boundary checks cover mixed stored history, explicit limits,
text-only requests, other models and unchanged incomplete responses. Fresh
immutable-candidate qualification remains necessary; rc11 does not contain this
correction. Provider-side correction would allow the original request to work
without a launcher-imposed limit.

The corrected development launcher also passed the original native-engine probe
without diagnostic request mutation: the engine still omitted the limit, the
adapter supplied it, and the image response completed with the correct colour
in 37.657 seconds. This is a native-engine integration result, not an Electron UI
pass. Regression coverage includes images returned by function and custom tools,
while ordinary text tool outputs remain unchanged.
