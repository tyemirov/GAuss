# GAuss

GAuss is a Google OAuth2 authentication package written in Go. It is designed to be embedded into other projects to easily authenticate users with Google and manage their sessions. A small demo application is provided under
`examples/user_auth` to illustrate how the package can be integrated.

---

## Features

- **OAuth2** with Google
- **Session Management** using [gorilla/sessions](https://github.com/gorilla/sessions)
- **Embeddable Templates** for the login page (default or custom)
- **Dashboard** showing user information after login
- **Reverse Proxy Awareness** that respects forwarded headers when computing redirects

---

## Getting Started

### Prerequisites

1. **Go** (version 1.23.4 or later recommended).
2. A **Google Cloud** project with OAuth credentials:
    - Client ID
    - Client Secret
3. A **session secret** (any random string or generated key).

### Configuring Google Cloud Console

1. Open [Google Cloud Console](https://console.cloud.google.com/) and select or create a project.
2. Navigate to **APIs & Services → Credentials** and click **Create credentials → OAuth client ID**.
3. Choose **Web application**.
4. Add `http://localhost:8080` under **Authorized JavaScript origins**.
5. Add `http://localhost:8080/auth/google/callback` under **Authorized redirect URIs**.
6. Save to obtain your **Client ID** and **Client Secret**. These values will be placed in your `.env` file.
7. If you plan to run the YouTube listing demo, enable the **YouTube Data API v3** for your project.

### Environment Variables

Set the following environment variables before running GAuss:

- `GOOGLE_CLIENT_ID` – Your Google OAuth2 client ID.
- `GOOGLE_CLIENT_SECRET` – Your Google OAuth2 client secret.
- `SESSION_SECRET` – The secret key for signing sessions.
- `PUBLIC_BASE_URL` – Optional external base URL used for redirect construction (`http://localhost:8080` by default).

For example, you might place them in an `.env` file (excluded from version control):

```bash
GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
GOOGLE_CLIENT_SECRET="your-google-client-secret"
SESSION_SECRET="random-secret"
PUBLIC_BASE_URL="http://localhost:8080"
# Callback URL configured in Google Cloud Console
# http://localhost:8080/auth/google/callback
```

### Run the Demo

This repository is not a standalone CLI tool. The code under `pkg/` is meant to
be imported into your own applications. However a small demonstration app lives
in `examples/user_auth` if you want to see GAuss in action.

1. **Clone** the repository or place the files in your Go workspace.
2. **Install** dependencies:
   ```bash
   go mod tidy
   ```
3. **Run** the demo application:
   ```bash
    go run examples/user_auth/main.go
   ```

The demo listens on `http://localhost:8080`.

There is also a YouTube listing demo under `examples/youtube_listing` that
requests the `youtube.readonly` scope and displays your uploaded videos.
Run it with:
```bash
go run examples/youtube_listing/main.go
```

---

## Custom Login Template

You can override the default embedded `login.html` in the demo by passing the
`--template` flag:

```bash
go run examples/user_auth/main.go --template="/path/to/your/custom_login.html"
```

- If the flag is **not** provided, GAuss uses its default embedded `login.html`.
- If the flag **is** provided, GAuss will parse your custom file and replace the embedded `login.html`.

### Example

```bash
go run examples/user_auth/main.go --template="templates/custom_login.html"
```

Ensure that your custom file exists and is accessible. Otherwise, you’ll get an error like
`template: pattern matches no files`.

---

## Usage

GAuss exposes packages under `pkg/` that you embed in your own Go programs. After setting the environment variables
`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` and `SESSION_SECRET`, create a `gauss.Service`, register its handlers with
your `http.ServeMux` and wrap protected routes with `gauss.AuthMiddleware`.

`NewService` now accepts the Google OAuth scopes you want to request. GAuss provides a set of scope constants and a
helper to convert them to strings:

```go
scopes := gauss.ScopeStrings([]gauss.Scope{gauss.ScopeProfile, gauss.ScopeEmail, gauss.ScopeYouTubeReadonly})
svc, err := gauss.NewService(clientID, clientSecret, baseURL, "/dashboard", scopes, "")
```

If the slice is empty, GAuss defaults to `profile` and `email`.

To see a working example, run the demo from `examples/user_auth`:

```bash
go run examples/user_auth/main.go
```

Open [http://localhost:8080/](http://localhost:8080/) and authenticate with Google. The demo demonstrates how to mount
the package’s handlers and how to serve a simple dashboard once the user is logged in.

---

## Reverse Proxy Support

GAuss recalculates the Google `redirect_uri` for every request by inspecting `Forwarded`,
`X-Forwarded-Proto`, `X-Forwarded-Host`, and `X-Forwarded-Port` headers. You can keep the demo
listening on HTTP behind a TLS terminator while still issuing HTTPS redirects to Google.
Ensure your proxy forwards those headers, for example with Nginx:

```nginx
location / {
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_pass http://localhost:8080;
}
```

Set `PUBLIC_BASE_URL` to your public host (the default `http://localhost:8080` works locally).
GAuss swaps the scheme or port automatically based on the forwarded metadata.

---

## Routes

- **`/login`** – Displays the login page (`login.html` or your custom file).
- **`/auth/google`** – Initiates Google OAuth2 flow.
- **`/auth/google/callback`** – Google redirects here with an authorization code.
- **`/logout`** – Logs out the user by clearing session data.
- **`/dashboard`** – Protected route showing user info.

### Customizing the Logout Redirect

GAuss redirects users to `/login` after logout by default. To send users back to a different landing page, pass the `gauss.WithLogoutRedirectURL` option when constructing the service:

```go
svc, err := gauss.NewService(
    clientID,
    clientSecret,
    publicBaseURL,
    "/dashboard",
    gauss.ScopeStrings(gauss.DefaultScopes),
    "",
    gauss.WithLogoutRedirectURL("/"),
)
```

If the option is omitted or the provided value is empty, GAuss continues redirecting to `/login`.

Because `/login` is the natural entry point for GAuss, many applications mount their public landing page there and simply redirect `/` to `/login`. That keeps login, post-auth, and logout flows aligned without extra plumbing:

```go
mux := http.NewServeMux()
mux.Handle("/", http.RedirectHandler("/login", http.StatusFound))
mux.Handle("/login", landingPageHandler) // renders your marketing page
mux = gaussHandlers.RegisterRoutes(mux)  // mounts /login, /auth/google, /callback, /logout
```

When you need to send users elsewhere after logout—such as an externally hosted marketing page—use `WithLogoutRedirectURL` to override the default.

### Persisting OAuth Tokens

After a successful login the raw OAuth2 token is stored in the session under the key `gauss.SessionKeyOAuthToken`. You
can extract and persist it for use outside the web session:

```go
sess, _ := session.Store().Get(r, constants.SessionName)
tokJSON, _ := sess.Values[constants.SessionKeyOAuthToken].(string)
var tok oauth2.Token
json.Unmarshal([]byte(tokJSON), &tok)
// save `tok` to your database
```

### Making Authenticated API Calls

The primary purpose of authenticating a user is to make API calls on their behalf. After retrieving the oauth2.Token
from the session, use the gauss.Service.GetClient method to create an *http.Client that is correctly configured to use
that token.

This authenticated client can then be passed to a Google API client library, such as the YouTube or Google Drive SDK.

#### Example:

```go
// Assume 'gaussSvc' is your initialized gauss.Service instance
// and 'r' is your http.Request.

// 1. Get the token from the session
sess, _ := session.Store().Get(r, constants.SessionName)
tokJSON, ok := sess.Values[constants.SessionKeyOAuthToken].(string)
if !ok {
   // Handle error: user not logged in or token is missing
   return
}
var token oauth2.Token
if err := json.Unmarshal([]byte(tokJSON), &token); err != nil {
   // Handle JSON parsing error
   return
}

// 2. Use the GAuss service to get an authenticated client
httpClient := gaussSvc.GetClient(r.Context(), &token)

// 3. Pass the client to a Google API library
youtubeService, err := youtube.NewService(r.Context(), option.WithHTTPClient(httpClient))
if err != nil {
   // Handle YouTube service creation error
   return
}

// 4. Use the service to make authenticated calls
channels, err := youtubeService.Channels.List([]string{"snippet"}).Mine(true).Do()
// ...
```

This approach ensures that the same OAuth2 configuration that initiated the login is used for all subsequent API calls,
preventing invalid_grant errors.

---

## Troubleshooting

1. **No custom file found**:  
   If you see `template: pattern matches no files`, ensure your custom template path is correct and accessible.
2. **State mismatch**:  
   If you see `error=invalid_state` in the URL, your session might have expired or your request was tampered with.
3. **Token exchange failed**:  
   Double-check your client ID and client secret, and that Google OAuth credentials are set correctly.

---

## License

GAuss project is licensed under the MIT License. See [LICENSE](MIT-LICENSE) for
details.

---

## Contributing

Feel free to open issues or pull requests. All contributions are welcome.

---

**Enjoy using GAuss for your Google OAuth2 authentication!**
