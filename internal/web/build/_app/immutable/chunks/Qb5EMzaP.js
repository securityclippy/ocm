const e=[{id:"discord",name:"Discord",category:"channel",description:"Discord bot for DMs and server channels",docsUrl:"https://docs.openclaw.ai/channels/discord",fields:[{name:"token",label:"Bot Token",envVar:"DISCORD_BOT_TOKEN",type:"password",placeholder:"MTIz...",required:!0,helpText:"From Discord Developer Portal → Bot → Token"}],elevationConfig:{readOnly:!1,defaultTTL:"24h"}},{id:"telegram",name:"Telegram",category:"channel",description:"Telegram bot via BotFather",docsUrl:"https://docs.openclaw.ai/channels/telegram",fields:[{name:"botToken",label:"Bot Token",envVar:"TELEGRAM_BOT_TOKEN",type:"password",placeholder:"123456:ABC-DEF...",required:!0,helpText:"From @BotFather on Telegram"}],elevationConfig:{readOnly:!1,defaultTTL:"24h"}},{id:"slack",name:"Slack (Bot App)",category:"channel",description:"Slack bot with socket mode",docsUrl:"https://docs.openclaw.ai/channels/slack",setupInstructions:`1. Go to api.slack.com/apps → Create New App → From scratch
2. Enable Socket Mode (left sidebar) → toggle ON
3. Basic Information → App-Level Tokens → Generate Token
   - Add scope: connections:write
   - Copy the App Token (xapp-...)
4. OAuth & Permissions → Add bot scopes:
   - channels:history, channels:read, chat:write
   - reactions:read, reactions:write, users:read
   - (add more as needed from the docs)
5. Install to Workspace → Copy Bot Token (xoxb-...)
6. Optional: Add User Token Scopes for expanded read access
   - Reinstall app → Copy User Token (xoxp-...)
7. Event Subscriptions → Enable → Subscribe to:
   - message.channels, message.groups, message.im
   - app_mention, reaction_added
8. App Home → Enable Messages Tab for DMs`,fields:[{name:"appToken",label:"App Token",envVar:"SLACK_APP_TOKEN",type:"password",placeholder:"xapp-1-...",required:!0,helpText:"Basic Information → App-Level Tokens (connections:write scope)"},{name:"botToken",label:"Bot Token",envVar:"SLACK_BOT_TOKEN",type:"password",placeholder:"xoxb-...",required:!0,helpText:"OAuth & Permissions → Bot User OAuth Token"},{name:"userToken",label:"User Token (optional)",envVar:"SLACK_USER_TOKEN",type:"password",placeholder:"xoxp-...",required:!1,helpText:"OAuth & Permissions → User OAuth Token (for history, pins, reactions, search)"}],elevationConfig:{readOnly:!1,defaultTTL:"24h"}},{id:"slack-personal",name:"Slack (Personal Token)",category:"integration",description:"Long-lived token to access Slack as yourself",docsUrl:"https://docs.openclaw.ai/channels/slack",setupInstructions:`Create a minimal personal app to get a long-lived user token:

1. Go to api.slack.com/apps → Create New App → From scratch
2. Name it anything (e.g., "My Personal Access")
3. OAuth & Permissions → User Token Scopes → Add these:
   - channels:history, channels:read (public channels)
   - groups:history, groups:read (private channels)
   - im:history, im:read (DMs)
   - mpim:history, mpim:read (group DMs)
   - users:read (user info)
   - search:read (search messages)
   - files:read (attachments)
4. Install to Workspace (top of OAuth page)
5. Copy the User OAuth Token (xoxp-...)

This token is long-lived and won't expire!
No bot needed - this acts as YOU.

---
Alternative: Browser token (short-lived, expires on logout)
- DevTools → Application → Cookies → copy "d" (xoxd-...)
- Console: JSON.parse(localStorage.localConfig_v2).teams[...].token (xoxc-...)`,fields:[{name:"userToken",label:"User Token",envVar:"SLACK_USER_TOKEN",type:"password",placeholder:"xoxp-... (recommended) or xoxc-...",required:!0,helpText:"xoxp- from OAuth app (long-lived) or xoxc- from browser (expires)"},{name:"cookie",label:"Cookie (only for xoxc tokens)",envVar:"SLACK_COOKIE",type:"password",placeholder:"xoxd-...",required:!1,helpText:"Required only if using browser xoxc- token"}],elevationConfig:{readOnly:!1,defaultTTL:"4h"}},{id:"openrouter",name:"OpenRouter",category:"provider",description:"Access to multiple LLM providers via OpenRouter",docsUrl:"https://openrouter.ai/docs",fields:[{name:"apiKey",label:"API Key",envVar:"OPENROUTER_API_KEY",type:"password",placeholder:"sk-or-...",required:!0}],elevationConfig:{readOnly:!0}},{id:"anthropic",name:"Anthropic",category:"provider",description:"Direct access to Claude models",docsUrl:"https://docs.anthropic.com",fields:[{name:"apiKey",label:"API Key",envVar:"ANTHROPIC_API_KEY",type:"password",placeholder:"sk-ant-...",required:!0}],elevationConfig:{readOnly:!0}},{id:"openai",name:"OpenAI",category:"provider",description:"Access to GPT models",docsUrl:"https://platform.openai.com/docs",fields:[{name:"apiKey",label:"API Key",envVar:"OPENAI_API_KEY",type:"password",placeholder:"sk-...",required:!0}],elevationConfig:{readOnly:!0}},{id:"groq",name:"Groq",category:"provider",description:"Fast inference with Groq",docsUrl:"https://console.groq.com/docs",fields:[{name:"apiKey",label:"API Key",envVar:"GROQ_API_KEY",type:"password",placeholder:"gsk_...",required:!0}],elevationConfig:{readOnly:!0}},{id:"brave",name:"Brave Search",category:"tool",description:"Web search via Brave Search API",docsUrl:"https://docs.openclaw.ai/brave-search",fields:[{name:"apiKey",label:"API Key",envVar:"BRAVE_API_KEY",type:"password",placeholder:"BSA...",required:!0,helpText:"From brave.com/search/api (use Data for Search plan)"}],elevationConfig:{readOnly:!0}},{id:"elevenlabs",name:"ElevenLabs",category:"tool",description:"Text-to-speech with ElevenLabs",docsUrl:"https://elevenlabs.io/docs",fields:[{name:"apiKey",label:"API Key",envVar:"ELEVENLABS_API_KEY",type:"password",required:!0}],elevationConfig:{readOnly:!0}},{id:"deepgram",name:"Deepgram",category:"tool",description:"Speech-to-text with Deepgram",docsUrl:"https://developers.deepgram.com",fields:[{name:"apiKey",label:"API Key",envVar:"DEEPGRAM_API_KEY",type:"password",required:!0}],elevationConfig:{readOnly:!0}},{id:"gmail",name:"Gmail",category:"integration",description:"Gmail read/send via Google OAuth (requires Google Cloud setup)",docsUrl:"https://docs.openclaw.ai/automation/gmail-pubsub",setupInstructions:`⚠️ Requires Google Cloud Console access (often blocked for work accounts)

Personal Gmail:
1. Create project at console.cloud.google.com
2. Enable Gmail API, create OAuth "Desktop app" credentials
3. Download client_secret.json

Then use gogcli:
  brew install steipete/tap/gogcli
  gog auth credentials ~/Downloads/client_secret.json
  gog auth add you@gmail.com

Work/Google Workspace:
  Ask IT admin to provision OAuth credentials or approve the app`,fields:[{name:"account",label:"Gmail Account",envVar:"GMAIL_ACCOUNT",type:"text",placeholder:"you@gmail.com",required:!0,helpText:"The Gmail account to use"},{name:"accessToken",label:"Access Token",envVar:"GMAIL_ACCESS_TOKEN",type:"password",required:!0,helpText:"From: cat ~/.config/gog/accounts/you@gmail.com.json"},{name:"refreshToken",label:"Refresh Token",envVar:"GMAIL_REFRESH_TOKEN",type:"password",required:!0,helpText:"From the same gog account JSON file"},{name:"clientId",label:"Client ID",envVar:"GMAIL_CLIENT_ID",type:"text",required:!0,helpText:"From your OAuth client_secret.json"},{name:"clientSecret",label:"Client Secret",envVar:"GMAIL_CLIENT_SECRET",type:"password",required:!0,helpText:"From your OAuth client_secret.json"}],elevationConfig:{readOnly:!1,defaultTTL:"1h"}},{id:"google-calendar",name:"Google Calendar",category:"integration",description:"Calendar access via Google OAuth (requires Google Cloud setup)",docsUrl:"https://gogcli.sh",setupInstructions:`⚠️ Requires Google Cloud Console access (often blocked for work accounts)

Personal account:
1. Create project at console.cloud.google.com
2. Enable Calendar API, create OAuth "Desktop app" credentials
3. Download client_secret.json

Then use gogcli:
  brew install steipete/tap/gogcli
  gog auth credentials ~/Downloads/client_secret.json
  gog auth add you@gmail.com --services calendar

Work/Google Workspace:
  Ask IT admin to provision OAuth credentials`,fields:[{name:"account",label:"Google Account",envVar:"GOOGLE_CALENDAR_ACCOUNT",type:"text",placeholder:"you@gmail.com",required:!0,helpText:"The Google account to use"},{name:"accessToken",label:"Access Token",envVar:"GOOGLE_CALENDAR_ACCESS_TOKEN",type:"password",required:!0,helpText:"From: cat ~/.config/gog/accounts/you@gmail.com.json"},{name:"refreshToken",label:"Refresh Token",envVar:"GOOGLE_CALENDAR_REFRESH_TOKEN",type:"password",required:!0,helpText:"From the same gog account JSON file"},{name:"clientId",label:"Client ID",envVar:"GOOGLE_CALENDAR_CLIENT_ID",type:"text",required:!0,helpText:"From your OAuth client_secret.json"},{name:"clientSecret",label:"Client Secret",envVar:"GOOGLE_CALENDAR_CLIENT_SECRET",type:"password",required:!0,helpText:"From your OAuth client_secret.json"}],elevationConfig:{readOnly:!0}},{id:"google-chat",name:"Google Chat",category:"channel",description:"Google Chat app via service account",docsUrl:"https://docs.openclaw.ai/channels/googlechat",fields:[{name:"serviceAccountFile",label:"Service Account JSON Path",envVar:"GOOGLE_CHAT_SERVICE_ACCOUNT_FILE",type:"text",placeholder:"~/.openclaw/googlechat-service-account.json",required:!0,helpText:"Path to the downloaded service account JSON file"}],elevationConfig:{readOnly:!1,defaultTTL:"24h"}},{id:"linear",name:"Linear",category:"integration",description:"Issue tracking with Linear",docsUrl:"https://linear.app/docs",fields:[{name:"apiKey",label:"API Key",envVar:"LINEAR_API_KEY",type:"password",required:!0,helpText:"From Linear Settings → API → Personal API Keys"}],elevationConfig:{readOnly:!1,defaultTTL:"1h"}},{id:"github",name:"GitHub",category:"integration",description:"GitHub API access",docsUrl:"https://docs.github.com/en/rest",fields:[{name:"token",label:"Personal Access Token",envVar:"GITHUB_TOKEN",type:"password",placeholder:"ghp_...",required:!0,helpText:"From GitHub Settings → Developer settings → Personal access tokens"}],elevationConfig:{readOnly:!1,defaultTTL:"1h"}},{id:"twitter",name:"Twitter / X",category:"integration",description:"Twitter API access",docsUrl:"https://developer.twitter.com/en/docs",fields:[{name:"bearerToken",label:"Bearer Token",envVar:"TWITTER_BEARER_TOKEN",type:"password",required:!1,helpText:"For read-only API access (v2 API)"},{name:"apiKey",label:"API Key",envVar:"TWITTER_API_KEY",type:"password",required:!1,helpText:"Consumer key for OAuth 1.0a"},{name:"apiSecret",label:"API Secret",envVar:"TWITTER_API_SECRET",type:"password",required:!1,helpText:"Consumer secret for OAuth 1.0a"},{name:"accessToken",label:"Access Token",envVar:"TWITTER_ACCESS_TOKEN",type:"password",required:!1,helpText:"User access token for posting"},{name:"accessSecret",label:"Access Token Secret",envVar:"TWITTER_ACCESS_SECRET",type:"password",required:!1,helpText:"User access token secret"}],elevationConfig:{readOnly:!1,defaultTTL:"1h"}},{id:"notion",name:"Notion",category:"integration",description:"Notion API access",docsUrl:"https://developers.notion.com",fields:[{name:"apiKey",label:"Integration Token",envVar:"NOTION_API_KEY",type:"password",placeholder:"secret_...",required:!0,helpText:"From Notion Settings → Integrations → Develop your own"}],elevationConfig:{readOnly:!1,defaultTTL:"1h"}}],o={channel:"Messaging Channels",provider:"AI/LLM Providers",tool:"Tool APIs",integration:"Integrations"};export{o as c,e as s};
