# Antigravity OAuth Constants

These are the **public** OAuth credentials used by the official Antigravity/Google CloudCode client.
They are embedded in the open-source client and are safe to use.

## OAuth Credentials
```
Client ID:     1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com
Client Secret: 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
Redirect URI:  http://localhost:51121/oauth-callback
```

## Required Scopes
```
https://www.googleapis.com/auth/cloud-platform
https://www.googleapis.com/auth/userinfo.email
https://www.googleapis.com/auth/userinfo.profile
https://www.googleapis.com/auth/cclog
https://www.googleapis.com/auth/experimentsandconfigs
```

## API Endpoints (in fallback order)
1. **Daily Sandbox**: `https://daily-cloudcode-pa.sandbox.googleapis.com`
2. **Autopush Sandbox**: `https://autopush-cloudcode-pa.sandbox.googleapis.com`
3. **Production**: `https://cloudcode-pa.googleapis.com`

## Required Headers

### Antigravity Requests
```http
User-Agent: antigravity/1.11.5 windows/amd64
X-Goog-Api-Client: google-cloud-sdk vscode_cloudshelleditor/0.1
Client-Metadata: {"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}
```

### Gemini CLI Requests
```http
User-Agent: google-api-nodejs-client/9.15.1
X-Goog-Api-Client: gl-node/22.17.0
Client-Metadata: ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI
```

## Default Project ID
When Antigravity does not return a project (e.g., for business/workspace accounts):
```
rising-fact-p41fc
```

## Special Constants

### Skip Thought Signature Validator
When a thinking block has an invalid/missing signature, use this sentinel to bypass validation:
```
skip_thought_signature_validator
```

### Empty Schema Placeholder
For tools with no parameters, add a placeholder to avoid empty schema errors:
```json
{
  "type": "object",
  "properties": {
    "_placeholder": {
      "type": "boolean",
      "description": "Placeholder. Always pass true."
    }
  },
  "required": ["_placeholder"]
}
```
