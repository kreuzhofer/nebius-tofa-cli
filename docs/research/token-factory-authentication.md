# Token Factory authentication and browser authorization

Research date: 2026-09-21. Tracks [Investigate Token Factory token/project authentication and browser authorization](https://github.com/kreuzhofer/nebius-tofa-cli/issues/10). Public documentation and source inspection only; no credentials, authenticated requests, browser sessions, registration, or inference calls were used.

## Finding

The documented direct inference credential is a **Token Factory API key sent as a Bearer token**. Inference and model-discovery references also expose the **`ai_project_id` query parameter**. This corroborates the token-plus-project request shape; it does not identify the issuer or credential type behind an existing user's “access token.” [API introduction](https://docs.tokenfactory.nebius.com/api-reference/introduction), [Chat Completions](https://docs.tokenfactory.nebius.com/api-reference/inference/create-chat-completion), [Responses](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response).

There is substantial evidence for browser authorization infrastructure: Token Factory documents a first-party CLI profile using its own federation host, and Nebius's public Go SDK implements Authorization Code with S256 PKCE and a loopback callback. **A supported independent public-client integration is still unestablished**: the inspected sources do not establish how this CLI obtains its own client registration, which delegated tokens inference accepts, or the required audience, permissions, project binding, renewal and consent contract. Missing documentation is not proof that the server lacks these capabilities. [Token Factory SSO](https://docs.tokenfactory.nebius.com/team-access/sso), [SDK authorization source](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/iam.go).

Recommendation: proceed with explicitly supplied Token Factory credentials and project configuration for the first prototype. Treat browser consent and return-to-CLI as a separate, conditional integration pending an approved client and authoritative backend contract. The engineering inquiry below makes that dependency concrete.

Evidence labels in this note: **documented** means an official product reference; **source-observed** means behavior in the pinned source; **inferred** combines those facts without a live check; **unverified** means the inspected evidence does not settle the question.

## Existing credentials and request contract

| Credential | Established use | Limit or unanswered question |
| --- | --- | --- |
| Token Factory API key | Created in the Token Factory console; shown once; supplied in `Authorization: Bearer …` to inference. | No expiration, refresh grant, delegated-consent issuance, or programmatic key-creation contract established from the inspected introduction. |
| Token Factory federation user token | Token Factory SSO documents official CLI access using `auth.tokenfactory.nebius.com`, with a Cloud control-plane endpoint. | Inference acceptance, audience, lifetime and public-client policy need confirmation. |
| Nebius Cloud user IAM access token | Cloud documents `nebius iam get-access-token` and a 12-hour lifetime. | This is a Cloud IAM statement, not a Token Factory API-key lifetime or universal inference credential. |
| Service-account authorized RSA key | Cloud signs an assertion with a private key and exchanges it for an IAM access token. | It is not a Token Factory console API key, an OAuth refresh token, or evidence of delegated user consent. |
| Cloud access key | Public IAM schema contains an AWS-style access-key ID and secret. | Distinct again; do not interpret every Nebius “key” as an inference Bearer credential. |

Sources: [TF API introduction](https://docs.tokenfactory.nebius.com/api-reference/introduction), [TF SSO](https://docs.tokenfactory.nebius.com/team-access/sso), [Cloud API authentication](https://docs.nebius.com/grpc-api/auth), [IAM access-key schema](https://github.com/nebius/api/blob/1396e2a05b3379967512d4d00d102e57705302dd/nebius/iam/v1/access_key.proto). The official [Cloud-and-Token-Factory workbench guidance](https://github.com/nebius/nebius-physical-ai/blob/main/docs/workbench/token-factory.md) also distinguishes the Token Factory API key from the Cloud IAM credential. That distinction does not establish rejection of every possible Token Factory-specific federated token.

The documented default API base is `https://api.tokenfactory.nebius.com/v1/`. A schematic request with explicit project selection is:

```http
GET /v1/models?ai_project_id=<project-id> HTTP/1.1
Host: api.tokenfactory.nebius.com
Authorization: Bearer <Token-Factory-credential>
```

`ai_project_id` is a query field in the references for `GET /v1/models`, `POST /v1/chat/completions`, and `POST /v1/responses`; it is not established here as an `OpenAI-Project` header or JSON-body field. The schema marks it optional, while the models reference also describes a 400 response for a missing or invalid project. Therefore a universal default-project rule cannot be inferred from optionality alone. The models reference describes 401 for missing authentication and 403 for insufficient access. [Models reference](https://docs.tokenfactory.nebius.com/api-reference/models/list-models), [Chat reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-chat-completion), [Responses reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response).

Keep credential type, endpoint and project explicit in configuration. A successful model-list request would validate only that credential's access to that operation/project; it would not establish permissions for every model or inference operation.

## Projects, permissions and regions

Token Factory organizes resources under organization → project. API keys are project resources, and a personal organization/default project is provisioned automatically. Projects support resources in multiple regions. The inspected organization page does not provide a public project-enumeration API. [Organizations and projects](https://docs.tokenfactory.nebius.com/team-access/org-projects).

Membership alone does not grant all project access. The documented project Admin and Member roles can access public endpoints; Admins can create/delete API keys, while Members can create/delete their own keys. Organization Admins access all projects; Billing Managers do not thereby obtain project-resource access. Service accounts appear in access management, but that does not establish their Token Factory inference credential-issuance procedure. [Groups and access management](https://docs.tokenfactory.nebius.com/team-access/groups). Access management itself is documented for all tiers. [Access overview](https://docs.tokenfactory.nebius.com/team-access/overview).

The dedicated-endpoint documentation separates a common control plane from regional inference hosts: `api.tokenfactory.nebius.com` for eu-north1, `api.tokenfactory.eu-west1.nebius.com` for eu-west1, and `api.tokenfactory.us-central1.nebius.com` for us-central1. This is a dedicated-endpoint routing table, not evidence of region-independent OAuth audiences or an equivalent serverless authentication policy. [Control and data planes](https://docs.tokenfactory.nebius.com/ai-models-inference/dedicated-endpoints/control-data-plane).

**Unverified:** API for enumerating accessible Token Factory organizations/projects; project-ID format beyond examples; default selection semantics; whether keys can span projects; scopes finer than group roles; region/account-tier restrictions on delegated tokens. The public Cloud `ProjectService.List(parent_id)` does not by itself establish enumeration of Token Factory projects. [Cloud project schema](https://github.com/nebius/api/blob/1396e2a05b3379967512d4d00d102e57705302dd/nebius/iam/v1/project_service.proto).

## Browser authorization: what exists and what it proves

Token Factory's SAML SSO setup explicitly uses official CLI profile fields `--endpoint api.nebius.cloud`, `--federation-endpoint auth.tokenfactory.nebius.com`, and an organization parent ID beginning `aitenant-`. The subsequent commands administer IAM federation resources. This is a concrete Token Factory/control-plane bridge, not merely a guess based on Cloud branding. SSO users still require group assignment. It does not document third-party inference consent or an application-registration workflow. [Token Factory SSO](https://docs.tokenfactory.nebius.com/team-access/sso).

Cloud's own CLI configuration opens a browser for user authentication and lets users select a tenant/project. Its usual federation host is `auth.nebius.com`. Its no-browser mode prints the authorization link; it is not evidence of the OAuth device authorization grant. A remote browser cannot automatically reach a callback bound on another machine. [CLI configuration](https://docs.nebius.com/cli/configure), [CLI without a browser](https://docs.nebius.com/cli/no-browser).

The inspected public Go SDK establishes the following mechanism; it is not a source review of the complete distributed Nebius CLI:

| Source-observed behavior | Evidence |
| --- | --- |
| Authorization request uses `response_type=code`, supplied `client_id`, `scope=openid`, random `state`, redirect URI, S256 challenge, and optional `federation-id`. | [Authorization helper](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/iam.go) |
| Token exchange is form POST with `grant_type=authorization_code`, code, redirect URI, client ID and verifier; this helper sends no client secret. Its response structure reads `access_token` and `expires_in`. | [Token helper](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/iam.go) |
| Callback listens on `127.0.0.1:0`, falling back to `[::1]:0`; code/state are checked before storing the returned code. | [Callback helper](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/callback.go) |
| Paths are fixed `/oauth2/authorize` and `/oauth2/token`; the file named `well_known.go` contains constants, not discovery retrieval. | [Endpoint constants](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/well_known.go) |
| SDK configuration requires a supplied client ID; the reader errors on an empty ID. | [Config reader](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/config/reader/reader.go), [reader options](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/config/reader/options.go) |
| Token acquisition stores expiry and can use a file cache; inspected federation acquisition has no refresh-token grant. | [Federation tokener](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation_token.go), [cache](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/file_cache.go) |
| Browser-launch helper handles macOS, Linux/WSL and Windows. | [Browser helper](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/browser/browser.go) |

**Inference, not a verified public contract:** combining the documented TF federation host with SDK path constants produces `https://auth.tokenfactory.nebius.com/oauth2/authorize` and `https://auth.tokenfactory.nebius.com/oauth2/token`. No authorization or token request was made. These source-derived addresses are not approval to choose arbitrary client IDs or ship against undocumented policy.

Nebius's Terraform provider documents a default profile client ID of `terraform-provider`. That establishes a first-party integration identity only. Do not reuse it as this CLI's identity. The official CLI's own client ID was not established from inspected source. [Terraform provider configuration](https://github.com/nebius/terraform-provider-nebius/blob/main/docs/index.md).

### Public support still needing confirmation

| Requirement | Current evidence status |
| --- | --- |
| Independent public/native-client registration and allowed redirects | Unverified; no registration procedure established. |
| Authoritative issuer/discovery metadata | Unverified for Token Factory; SDK uses configured host plus fixed paths. |
| Public PKCE client accepted by TF inference | Unverified; first-party helper and TF federation docs establish infrastructure, not this permission. |
| Inference audience and least-privilege scopes | Unverified; SDK's `openid` is not an established inference permission. |
| Consent UX, project selection and resulting token/project binding | Unverified. |
| Refresh tokens, `offline_access`, rotation and expiry | Unverified for TF; absent in inspected helper, not proven absent server-side. |
| RFC 8628 device authorization | Unverified; printed browser link is not device flow. |
| Per-application revocation, key rotation and logout semantics | Only partial evidence below. |
| Personal/social-login versus enterprise SAML availability | TF quickstart and SSO describe different account paths; a common delegated-client contract is unverified. |

Token Factory's [quickstart](https://docs.tokenfactory.nebius.com/quickstart) describes Google/GitHub console sign-in followed by API-key creation. Signing into that console does not by itself authorize an independent CLI.

## Renewal, exchange and revocation

Cloud IAM documentation gives user access tokens a 12-hour lifetime and describes service-account JWT exchange (a short-lived signed assertion producing an access token). Do not transplant these values to TF keys or federated tokens without confirmation. [Cloud authentication](https://docs.nebius.com/grpc-api/auth).

The public IAM token schema defines RFC 8693 token exchange, including subject/actor tokens, requested scopes, resource and audience; the audience comment describes an OAuth client ID. It returns an access token and expiry, with no refresh token in that response schema. This is evidence for an exchange facility, not the required Token Factory audience/scopes or an inference authorization recipe. [Token schema](https://github.com/nebius/api/blob/1396e2a05b3379967512d4d00d102e57705302dd/nebius/iam/v1/token_service.proto), [exchange service](https://github.com/nebius/api/blob/1396e2a05b3379967512d4d00d102e57705302dd/nebius/iam/v1/token_exchange_service.proto).

IAM session management can revoke all tokens for a service account, all active sessions/access tokens for the current user, or a tenant user account's sessions/tokens. These broad operations are not evidence of a narrowly scoped OAuth revoke endpoint. They should not silently implement this CLI's logout. Token Factory roles separately document API-key deletion rights, but the inspected pages do not settle propagation delay or delegated-token revocation. [Session management schema](https://github.com/nebius/api/blob/1396e2a05b3379967512d4d00d102e57705302dd/nebius/iam/v1/session_management_service.proto), [TF key permissions](https://docs.tokenfactory.nebius.com/team-access/groups).

## Implementation direction and verification gates

For a supported browser flow, use an independently registered public native client, an external browser, Authorization Code with S256 PKCE and a registered loopback redirect. A distributable CLI cannot protect an embedded shared client secret. These are standards-based recommendations, **not claims of Token Factory support**. [RFC 8252](https://www.rfc-editor.org/rfc/rfc8252). Offer device authorization for remote/headless use only if Nebius confirms the separate endpoint/grant and polling/error contract. [RFC 8628](https://www.rfc-editor.org/rfc/rfc8628).

The Go SDK is [MIT licensed](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/LICENSE), but do not copy its helper unreviewed. At this pin, its verifier is 32 characters, while RFC 7636 requires 43–128; the token helper decodes JSON without checking HTTP status; a bad-state callback ends the wait without saving a code, aborting rather than accepting authentication. These are bounded source-review observations, not an audit of Nebius's server. [PKCE helper](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/pkce.go), [RFC 7636 §4.1](https://www.rfc-editor.org/rfc/rfc7636#section-4.1), [token helper](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/iam.go), [callback](https://github.com/nebius/gosdk/blob/90d2276a875fe5670b8e07a937a1248de5b0e856/auth/federation/callback.go).

Proposed verification sequence, after backend confirmation and explicitly provided test credentials:

1. Obtain issuer/endpoints, this application's registered client ID, redirect policy, scopes/audience, and test account/project contract. Record which account types and regions it covers.
2. Verify browser success, denied consent, timeout, callback state mismatch, invalid/reused code and wrong verifier on macOS, Windows and Linux. Confirm no secret is embedded in the binary.
3. Verify project listing/selection and non-billable model discovery for an authorized project; verify denied access for a different project. Confirm omitted-project behavior separately for each accepted token type.
4. Verify expiration, refresh if supported, permission removal and application-specific revoke. Local credential deletion and server revocation must be distinct operations with explicit behavior.
5. Verify downstream agent credential injection and expiry handling independently. A launcher obtaining a token does not establish that a long-running child agent can refresh it. Any inference validation requiring spend remains a separately authorized test.

For the supplied-credential prototype, use explicit endpoint/project settings, avoid persisting secrets in repository files, and report authentication failure separately from missing project permissions. Do not ingest existing browser cookies or first-party CLI credential caches. Those are implementation recommendations, not discovered API features.

## Draft inquiry to Nebius engineering — not sent

We are building a standalone macOS/Windows/Linux launcher for local coding agents using Token Factory directly. The initial prototype accepts a supplied credential plus `ai_project_id`. We want an optional browser flow where users see this app and its requested permissions, select a project, consent, and return to the CLI without copying a key.

Public evidence shows Token Factory's first-party CLI federation host (`auth.tokenfactory.nebius.com`) and a Nebius SDK code/PKCE loopback flow. Could you confirm the supported **independent public/native-client** contract rather than having us reuse a first-party client ID?

1. Which credentials may call `/v1/models`, `/v1/chat/completions` and `/v1/responses`: TF API keys, TF-federated user access tokens, exchanged tokens, service-account tokens? Please specify issuer, audience, scopes and `ai_project_id` placement/default rules for each.
2. Is Authorization Code + S256 PKCE supported for independently distributed CLIs? How do we register our app/client ID, allowed loopback redirects and dynamic ports, consent text, scopes and audience? Please supply authoritative issuer/discovery metadata or endpoint documentation.
3. Can authorization be limited to a selected project and inference/model discovery, without key-management or organization-admin privileges? How do we list accessible organizations/projects and show their names? When and how is project choice bound to the token?
4. Which access-token lifetimes, refresh-token grants/rotation policies, expiry errors and per-application revocation mechanisms apply? What happens after membership removal? How should a launcher refresh credentials used by long-running child agents?
5. Is RFC 8628 device authorization supported for remote/headless machines? If so, please provide endpoints, supported scopes/audiences, polling/error behavior and client registration requirements.
6. Are personal Google/GitHub accounts, enterprise SAML users, service accounts, organizations and regional endpoints covered by the same contract? Which combinations require separate configuration?
7. If this is internal-only today, can you provide the supported onboarding route or define the missing backend capability? We need an approved public-client identity, consent/project authorization, inference acceptance and lifecycle contract before committing browser login to the product scope. No delivery estimate is assumed.

## Source boundary and reproducibility

Public documentation was reviewed on the research date; those pages and provider docs can change. SDK observations are pinned to `nebius/gosdk` commit `90d2276a875fe5670b8e07a937a1248de5b0e856`; API schema observations to `nebius/api` commit `1396e2a05b3379967512d4d00d102e57705302dd`. Searches of the inspected SDK auth/config/browser packages did not establish refresh, device authorization, dynamic registration or discovery implementations. That is a bounded source result, not a statement that the authorization server lacks them.

No public full-CLI source was established during this inspection, and no network behavior of the authorization service was tested. No proxy/adapter work is included. This research resolves what can currently be evidenced and identifies the engineering answers required to implement browser login; it does not claim browser login is already publicly supported or impossible.
