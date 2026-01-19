# Antigravity API Specification

## Endpoints
- **Production**: `https://cloudcode-pa.googleapis.com`
- **Sandbox**: `https://daily-cloudcode-pa.sandbox.googleapis.com`

## Required Scopes
- `https://www.googleapis.com/auth/cloud-platform`
- `https://www.googleapis.com/auth/userinfo.email`
- `https://www.googleapis.com/auth/userinfo.profile`
- `https://www.googleapis.com/auth/cclog`
- `https://www.googleapis.com/auth/experimentsandconfigs`

## Headers
```http
Authorization: Bearer {access_token}
Content-Type: application/json
User-Agent: antigravity/1.11.5 windows/amd64
X-Goog-Api-Client: google-cloud-sdk vscode_cloudshelleditor/0.1
Client-Metadata: {"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}
```

## Request Format (Gemini Style)
Even for Claude models, the request structure must follow the Gemini `contents` array format, NOT the Anthropic `messages` format.

```json
{
  "project": "{project_id}",
  "model": "{model_id}", // e.g., "claude-sonnet-4-5"
  "request": {
    "contents": [
      {
        "role": "user",
        "parts": [{ "text": "Hello" }]
      }
    ],
    "generationConfig": {
        "maxOutputTokens": 1000,
        "thinkingConfig": {
            "thinkingBudget": 4000,
            "includeThoughts": true
        }
    }
  },
  "userAgent": "antigravity",
  "requestId": "{unique_id}"
}
```

## Supported Models
- `claude-sonnet-4-5`
- `claude-sonnet-4-5-thinking`
- `claude-opus-4-5-thinking`
- `gemini-3-pro-high`
- `gemini-3-pro-low`
