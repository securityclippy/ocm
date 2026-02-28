// Service templates for OCM credential management
// These define the expected fields, injection targets, and docs for each known service

// Injection target types - where credentials get written
export type InjectionTarget =
	| { type: 'env'; var: string }           // Write to .env file
	| { type: 'config'; path: string };      // Patch OpenClaw config file

export interface FieldConfig {
	name: string;
	label: string;
	type: 'text' | 'password' | 'textarea';
	placeholder?: string;
	required?: boolean;
	helpText?: string;
	// Injection target - where this field's value gets written
	// Can be env var, config path, or both (for backwards compat, envVar alone works)
	injection?: InjectionTarget;
	envVar?: string;  // Legacy: equivalent to injection: { type: 'env', var: envVar }
}

export interface ServiceTemplate {
	id: string;
	name: string;
	category: 'channel' | 'provider' | 'tool' | 'integration';
	description: string;
	docsUrl?: string;
	setupInstructions?: string; // Multi-line setup steps
	fields: FieldConfig[];
	// Elevation config: which operations need approval
	elevationConfig?: {
		readOnly?: boolean; // If true, no elevation needed (just API reads)
		defaultTTL?: string; // Default elevation TTL
	};
}

// Helper to get injection target from a field (handles legacy envVar)
export function getFieldInjection(field: FieldConfig): InjectionTarget | undefined {
	if (field.injection) {
		return field.injection;
	}
	if (field.envVar) {
		return { type: 'env', var: field.envVar };
	}
	return undefined;
}

export const serviceTemplates: ServiceTemplate[] = [
	// ===== Messaging Channels =====
	{
		id: 'discord',
		name: 'Discord',
		category: 'channel',
		description: 'Discord bot for DMs and server channels',
		docsUrl: 'https://docs.openclaw.ai/channels/discord',
		fields: [
			{
				name: 'token',
				label: 'Bot Token',
				envVar: 'DISCORD_BOT_TOKEN',
				type: 'password',
				placeholder: 'MTIz...',
				required: true,
				helpText: 'From Discord Developer Portal → Bot → Token'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '24h' }
	},
	{
		id: 'telegram',
		name: 'Telegram',
		category: 'channel',
		description: 'Telegram bot via BotFather',
		docsUrl: 'https://docs.openclaw.ai/channels/telegram',
		fields: [
			{
				name: 'botToken',
				label: 'Bot Token',
				envVar: 'TELEGRAM_BOT_TOKEN',
				type: 'password',
				placeholder: '123456:ABC-DEF...',
				required: true,
				helpText: 'From @BotFather on Telegram'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '24h' }
	},
	{
		id: 'slack',
		name: 'Slack (Bot App)',
		category: 'channel',
		description: 'Slack bot with socket mode',
		docsUrl: 'https://docs.openclaw.ai/channels/slack',
		setupInstructions: `1. Go to api.slack.com/apps → Create New App → From scratch
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
8. App Home → Enable Messages Tab for DMs`,
		fields: [
			{
				name: 'appToken',
				label: 'App Token',
				envVar: 'SLACK_APP_TOKEN',
				type: 'password',
				placeholder: 'xapp-1-...',
				required: true,
				helpText: 'Basic Information → App-Level Tokens (connections:write scope)'
			},
			{
				name: 'botToken',
				label: 'Bot Token',
				envVar: 'SLACK_BOT_TOKEN',
				type: 'password',
				placeholder: 'xoxb-...',
				required: true,
				helpText: 'OAuth & Permissions → Bot User OAuth Token'
			},
			{
				name: 'userToken',
				label: 'User Token (optional)',
				type: 'password',
				placeholder: 'xoxp-...',
				required: false,
				helpText: 'OAuth & Permissions → User OAuth Token (for reading your messages)',
				// User token goes to config file, not env var
				injection: { type: 'config', path: 'channels.slack.userToken' }
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '24h' }
	},
	{
		id: 'slack-personal',
		name: 'Slack (Personal Token)',
		category: 'integration',
		description: 'Long-lived token to access Slack as yourself',
		docsUrl: 'https://docs.openclaw.ai/channels/slack',
		setupInstructions: `═══════════════════════════════════════════════════
OPTION 1: OAuth App (Recommended - Never Expires)
═══════════════════════════════════════════════════
1. Go to api.slack.com/apps → Create New App → From scratch
2. Name it anything (e.g., "My Personal Access")
3. OAuth & Permissions → User Token Scopes → Add:
   - channels:history, channels:read
   - groups:history, groups:read
   - im:history, im:read
   - mpim:history, mpim:read
   - users:read, search:read, files:read
4. Install to Workspace
5. Copy User OAuth Token → paste below

   ✅ Just need: User Token (xoxp-...)
   ❌ Cookie: Not needed

═══════════════════════════════════════════════════
OPTION 2: Browser Token (Quick but Expires on Logout)
═══════════════════════════════════════════════════
⚠️  REQUIRES BOTH TOKEN AND COOKIE

Open Slack in browser → DevTools (F12):

1. GET THE TOKEN:
   Console tab → paste this:
   JSON.parse(localStorage.localConfig_v2).teams[Object.keys(JSON.parse(localStorage.localConfig_v2).teams)[0]].token
   
   Copy the result (starts with xoxc-...)

2. GET THE COOKIE:
   Application tab → Cookies → slack.com
   Find the "d" cookie → copy its value (starts with xoxd-...)

   ✅ Need BOTH: User Token (xoxc-...) AND Cookie (xoxd-...)`,
		fields: [
			{
				name: 'userToken',
				label: 'User Token',
				type: 'password',
				placeholder: 'xoxp-... or xoxc-...',
				required: true,
				helpText: 'xoxp- from OAuth app, or xoxc- from browser (requires cookie below)',
				injection: { type: 'config', path: 'channels.slack.userToken' }
			},
			{
				name: 'cookie',
				label: 'Cookie (required for xoxc- tokens)',
				type: 'password',
				placeholder: 'xoxd-...',
				required: false,
				helpText: 'Only needed for browser tokens (xoxc-). The "d" cookie value from slack.com',
				injection: { type: 'config', path: 'channels.slack.cookie' }
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '4h' }
	},

	// ===== AI/LLM Providers =====
	{
		id: 'openrouter',
		name: 'OpenRouter',
		category: 'provider',
		description: 'Access to multiple LLM providers via OpenRouter',
		docsUrl: 'https://openrouter.ai/docs',
		fields: [
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'OPENROUTER_API_KEY',
				type: 'password',
				placeholder: 'sk-or-...',
				required: true
			}
		],
		elevationConfig: { readOnly: true } // Just API calls, no approval needed
	},
	{
		id: 'anthropic',
		name: 'Anthropic',
		category: 'provider',
		description: 'Direct access to Claude models',
		docsUrl: 'https://console.anthropic.com',
		setupInstructions: `Option 1: API Key (recommended for API access)
1. Go to console.anthropic.com → API Keys
2. Create a new key and copy it (sk-ant-...)

Option 2: Claude Code Token (for Claude Pro/Max subscribers)
If you have a Claude subscription and Claude Code CLI installed:
1. Run: claude setup-token
2. Follow the prompts to authenticate
3. Copy the token that's generated

The setup-token is long-lived and works with your subscription.
Use an API key if you need standard API access with usage-based billing.`,
		fields: [
			{
				name: 'apiKey',
				label: 'API Key or Setup Token',
				envVar: 'ANTHROPIC_API_KEY',
				type: 'password',
				placeholder: 'sk-ant-... or setup token',
				required: true,
				helpText: 'API key from console.anthropic.com OR token from `claude setup-token`'
			}
		],
		elevationConfig: { readOnly: true }
	},
	{
		id: 'openai',
		name: 'OpenAI',
		category: 'provider',
		description: 'Access to GPT models',
		docsUrl: 'https://platform.openai.com/docs',
		fields: [
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'OPENAI_API_KEY',
				type: 'password',
				placeholder: 'sk-...',
				required: true
			}
		],
		elevationConfig: { readOnly: true }
	},
	{
		id: 'groq',
		name: 'Groq',
		category: 'provider',
		description: 'Fast inference with Groq',
		docsUrl: 'https://console.groq.com/docs',
		fields: [
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'GROQ_API_KEY',
				type: 'password',
				placeholder: 'gsk_...',
				required: true
			}
		],
		elevationConfig: { readOnly: true }
	},

	// ===== Tool APIs =====
	{
		id: 'brave',
		name: 'Brave Search',
		category: 'tool',
		description: 'Web search via Brave Search API',
		docsUrl: 'https://docs.openclaw.ai/brave-search',
		fields: [
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'BRAVE_API_KEY',
				type: 'password',
				placeholder: 'BSA...',
				required: true,
				helpText: 'From brave.com/search/api (use Data for Search plan)'
			}
		],
		elevationConfig: { readOnly: true }
	},
	{
		id: 'elevenlabs',
		name: 'ElevenLabs',
		category: 'tool',
		description: 'Text-to-speech with ElevenLabs',
		docsUrl: 'https://elevenlabs.io/docs',
		fields: [
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'ELEVENLABS_API_KEY',
				type: 'password',
				required: true
			}
		],
		elevationConfig: { readOnly: true }
	},
	{
		id: 'deepgram',
		name: 'Deepgram',
		category: 'tool',
		description: 'Speech-to-text with Deepgram',
		docsUrl: 'https://developers.deepgram.com',
		fields: [
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'DEEPGRAM_API_KEY',
				type: 'password',
				required: true
			}
		],
		elevationConfig: { readOnly: true }
	},

	// ===== Google Services =====
	// Note: Google OAuth requires a Google Cloud project with OAuth credentials.
	// This is straightforward for personal accounts but often blocked for work accounts
	// (requires IT admin to provision OAuth clients or approve third-party apps).
	{
		id: 'gmail',
		name: 'Gmail',
		category: 'integration',
		description: 'Gmail read/send via Google OAuth (requires Google Cloud setup)',
		docsUrl: 'https://docs.openclaw.ai/automation/gmail-pubsub',
		setupInstructions: `⚠️ Requires Google Cloud Console access (often blocked for work accounts)

Personal Gmail:
1. Create project at console.cloud.google.com
2. Enable Gmail API, create OAuth "Desktop app" credentials
3. Download client_secret.json

Then use gogcli:
  brew install steipete/tap/gogcli
  gog auth credentials ~/Downloads/client_secret.json
  gog auth add you@gmail.com

Work/Google Workspace:
  Ask IT admin to provision OAuth credentials or approve the app`,
		fields: [
			{
				name: 'account',
				label: 'Gmail Account',
				envVar: 'GMAIL_ACCOUNT',
				type: 'text',
				placeholder: 'you@gmail.com',
				required: true,
				helpText: 'The Gmail account to use'
			},
			{
				name: 'accessToken',
				label: 'Access Token',
				envVar: 'GMAIL_ACCESS_TOKEN',
				type: 'password',
				required: true,
				helpText: 'From: cat ~/.config/gog/accounts/you@gmail.com.json'
			},
			{
				name: 'refreshToken',
				label: 'Refresh Token',
				envVar: 'GMAIL_REFRESH_TOKEN',
				type: 'password',
				required: true,
				helpText: 'From the same gog account JSON file'
			},
			{
				name: 'clientId',
				label: 'Client ID',
				envVar: 'GMAIL_CLIENT_ID',
				type: 'text',
				required: true,
				helpText: 'From your OAuth client_secret.json'
			},
			{
				name: 'clientSecret',
				label: 'Client Secret',
				envVar: 'GMAIL_CLIENT_SECRET',
				type: 'password',
				required: true,
				helpText: 'From your OAuth client_secret.json'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '1h' }
	},
	{
		id: 'google-calendar',
		name: 'Google Calendar',
		category: 'integration',
		description: 'Calendar access via Google OAuth (requires Google Cloud setup)',
		docsUrl: 'https://gogcli.sh',
		setupInstructions: `⚠️ Requires Google Cloud Console access (often blocked for work accounts)

Personal account:
1. Create project at console.cloud.google.com
2. Enable Calendar API, create OAuth "Desktop app" credentials
3. Download client_secret.json

Then use gogcli:
  brew install steipete/tap/gogcli
  gog auth credentials ~/Downloads/client_secret.json
  gog auth add you@gmail.com --services calendar

Work/Google Workspace:
  Ask IT admin to provision OAuth credentials`,
		fields: [
			{
				name: 'account',
				label: 'Google Account',
				envVar: 'GOOGLE_CALENDAR_ACCOUNT',
				type: 'text',
				placeholder: 'you@gmail.com',
				required: true,
				helpText: 'The Google account to use'
			},
			{
				name: 'accessToken',
				label: 'Access Token',
				envVar: 'GOOGLE_CALENDAR_ACCESS_TOKEN',
				type: 'password',
				required: true,
				helpText: 'From: cat ~/.config/gog/accounts/you@gmail.com.json'
			},
			{
				name: 'refreshToken',
				label: 'Refresh Token',
				envVar: 'GOOGLE_CALENDAR_REFRESH_TOKEN',
				type: 'password',
				required: true,
				helpText: 'From the same gog account JSON file'
			},
			{
				name: 'clientId',
				label: 'Client ID',
				envVar: 'GOOGLE_CALENDAR_CLIENT_ID',
				type: 'text',
				required: true,
				helpText: 'From your OAuth client_secret.json'
			},
			{
				name: 'clientSecret',
				label: 'Client Secret',
				envVar: 'GOOGLE_CALENDAR_CLIENT_SECRET',
				type: 'password',
				required: true,
				helpText: 'From your OAuth client_secret.json'
			}
		],
		elevationConfig: { readOnly: true }
	},
	{
		id: 'google-chat',
		name: 'Google Chat',
		category: 'channel',
		description: 'Google Chat app via service account',
		docsUrl: 'https://docs.openclaw.ai/channels/googlechat',
		fields: [
			{
				name: 'serviceAccountFile',
				label: 'Service Account JSON Path',
				envVar: 'GOOGLE_CHAT_SERVICE_ACCOUNT_FILE',
				type: 'text',
				placeholder: '~/.openclaw/googlechat-service-account.json',
				required: true,
				helpText: 'Path to the downloaded service account JSON file'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '24h' }
	},

	// ===== Integrations =====
	{
		id: 'linear',
		name: 'Linear',
		category: 'integration',
		description: 'Issue tracking with Linear',
		docsUrl: 'https://linear.app/docs',
		fields: [
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'LINEAR_API_KEY',
				type: 'password',
				required: true,
				helpText: 'From Linear Settings → API → Personal API Keys'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '1h' } // Creating issues needs approval
	},
	{
		id: 'github',
		name: 'GitHub',
		category: 'integration',
		description: 'GitHub API access',
		docsUrl: 'https://docs.github.com/en/rest',
		fields: [
			{
				name: 'token',
				label: 'Personal Access Token',
				envVar: 'GITHUB_TOKEN',
				type: 'password',
				placeholder: 'ghp_...',
				required: true,
				helpText: 'From GitHub Settings → Developer settings → Personal access tokens'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '1h' }
	},
	{
		id: 'twitter',
		name: 'Twitter / X',
		category: 'integration',
		description: 'Twitter API access',
		docsUrl: 'https://developer.twitter.com/en/docs',
		fields: [
			{
				name: 'bearerToken',
				label: 'Bearer Token',
				envVar: 'TWITTER_BEARER_TOKEN',
				type: 'password',
				required: false,
				helpText: 'For read-only API access (v2 API)'
			},
			{
				name: 'apiKey',
				label: 'API Key',
				envVar: 'TWITTER_API_KEY',
				type: 'password',
				required: false,
				helpText: 'Consumer key for OAuth 1.0a'
			},
			{
				name: 'apiSecret',
				label: 'API Secret',
				envVar: 'TWITTER_API_SECRET',
				type: 'password',
				required: false,
				helpText: 'Consumer secret for OAuth 1.0a'
			},
			{
				name: 'accessToken',
				label: 'Access Token',
				envVar: 'TWITTER_ACCESS_TOKEN',
				type: 'password',
				required: false,
				helpText: 'User access token for posting'
			},
			{
				name: 'accessSecret',
				label: 'Access Token Secret',
				envVar: 'TWITTER_ACCESS_SECRET',
				type: 'password',
				required: false,
				helpText: 'User access token secret'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '1h' }
	},
	{
		id: 'notion',
		name: 'Notion',
		category: 'integration',
		description: 'Notion API access',
		docsUrl: 'https://developers.notion.com',
		fields: [
			{
				name: 'apiKey',
				label: 'Integration Token',
				envVar: 'NOTION_API_KEY',
				type: 'password',
				placeholder: 'secret_...',
				required: true,
				helpText: 'From Notion Settings → Integrations → Develop your own'
			}
		],
		elevationConfig: { readOnly: false, defaultTTL: '1h' }
	}
];

export const categoryLabels: Record<string, string> = {
	channel: 'Messaging Channels',
	provider: 'AI/LLM Providers',
	tool: 'Tool APIs',
	integration: 'Integrations'
};

export function getTemplateById(id: string): ServiceTemplate | undefined {
	return serviceTemplates.find(t => t.id === id);
}

export function getTemplatesByCategory(category: string): ServiceTemplate[] {
	return serviceTemplates.filter(t => t.category === category);
}
