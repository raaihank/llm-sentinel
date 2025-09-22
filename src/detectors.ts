export interface MaskingRule {
  name: string;
  pattern: RegExp;
  replacement: string | ((match: string, ...groups: string[]) => string);
}

export class SensitiveDataDetector {
  private rules: MaskingRule[];

  constructor() {
    this.rules = this.getDefaultRules();
  }

  private getDefaultRules(): MaskingRule[] {
    return [
      {
        name: 'userPath',
        pattern: /((?:\/Users\/|\/home\/|C:\\Users\\))([^\/\\\s"']+)((?:[\/\\][^\s"']*)?)/gi,
        replacement: (_match: string, prefix: string, _username: string, suffix: string) => {
          return `${prefix}[USERNAME]${suffix}`;
        }
      },
      {
        name: 'apiKey',
        pattern: /(?:api[_-]?key|apikey)["']?\s*[:=]\s*["']?[a-zA-Z0-9\-_.~+\/]+=*["']?/gi,
        replacement: '[API_KEY_MASKED]'
      },
      {
        name: 'secret',
        pattern: /(?:secret|token|password|passwd|pwd)["']?\s*[:=]\s*["']?[a-zA-Z0-9\-_.~+\/!@#$%^&*]+=*["']?/gi,
        replacement: '[SECRET_MASKED]'
      },
      {
        name: 'openaiApiKey',
        pattern: /sk-[a-zA-Z0-9]{20,}/g,
        replacement: '[OPENAI_API_KEY_MASKED]'
      },
      {
        name: 'openaiOrgId',
        pattern: /org-[a-zA-Z0-9]{20,}/g,
        replacement: '[OPENAI_ORG_ID_MASKED]'
      },
      {
        name: 'openaiProjectId',
        pattern: /sk-proj-[a-zA-Z0-9]{20,}/g,
        replacement: '[OPENAI_PROJECT_KEY_MASKED]'
      },
      {
        name: 'anthropicApiKey',
        pattern: /sk-ant-[a-zA-Z0-9]{20,}/g,
        replacement: '[ANTHROPIC_API_KEY_MASKED]'
      },
      {
        name: 'claudeApiKey',
        pattern: /sk-ant-[a-zA-Z0-9]{20,}/g,
        replacement: '[CLAUDE_API_KEY_MASKED]'
      },
      {
        name: 'googleCloudApiKey',
        pattern: /AIza[0-9A-Za-z\-_]{35}/g,
        replacement: '[GOOGLE_CLOUD_API_KEY_MASKED]'
      },
      {
        name: 'googleCloudServiceAccount',
        pattern: /[a-zA-Z0-9\-_]+@[a-zA-Z0-9\-_]+\.iam\.gserviceaccount\.com/g,
        replacement: '[GOOGLE_CLOUD_SERVICE_ACCOUNT_MASKED]'
      },
      {
        name: 'googleCloudProjectId',
        pattern: /"project_id":\s*"([a-zA-Z0-9\-_]{6,30})"/g,
        replacement: '"project_id": "[GOOGLE_CLOUD_PROJECT_ID_MASKED]"'
      },
      {
        name: 'googleCloudCredentials',
        pattern: /"type":\s*"service_account".*?"private_key":\s*"-----BEGIN PRIVATE KEY-----[^"]*-----END PRIVATE KEY-----"/gs,
        replacement: '[GOOGLE_CLOUD_CREDENTIALS_MASKED]'
      },
      {
        name: 'azureOpenaiApiKey',
        pattern: /(?:api[_-]?key|apikey)["']?\s*[:=]\s*["']?[a-zA-Z0-9]{32}["']?/gi,
        replacement: '[AZURE_OPENAI_API_KEY_MASKED]'
      },
      {
        name: 'cohereApiKey',
        pattern: /(?:cohere[_-]?api[_-]?key|cohere[_-]?key)["']?\s*[:=]\s*["']?[a-zA-Z0-9]{40}["']?/gi,
        replacement: '[COHERE_API_KEY_MASKED]'
      },
      {
        name: 'huggingfaceToken',
        pattern: /(?:hf[_-]?token|huggingface[_-]?token)["']?\s*[:=]\s*["']?hf_[a-zA-Z0-9]{39}["']?/gi,
        replacement: '[HUGGINGFACE_TOKEN_MASKED]'
      },
      {
        name: 'pineconeApiKey',
        pattern: /(?:pinecone[_-]?api[_-]?key|pinecone[_-]?key)["']?\s*[:=]\s*["']?[a-zA-Z0-9]{32}["']?/gi,
        replacement: '[PINECONE_API_KEY_MASKED]'
      },
      {
        name: 'weaviateApiKey',
        pattern: /(?:weaviate[_-]?api[_-]?key|weaviate[_-]?key)["']?\s*[:=]\s*["']?[a-zA-Z0-9]{32}["']?/gi,
        replacement: '[WEAVIATE_API_KEY_MASKED]'
      },
      {
        name: 'chromaApiKey',
        pattern: /(?:chroma[_-]?api[_-]?key|chroma[_-]?key)["']?\s*[:=]\s*["']?[a-zA-Z0-9]{32}["']?/gi,
        replacement: '[CHROMA_API_KEY_MASKED]'
      },
      {
        name: 'email',
        pattern: /[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/g,
        replacement: '[EMAIL_MASKED]'
      },
      {
        name: 'ipAddress',
        pattern: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g,
        replacement: '[IP_MASKED]'
      },
      {
        name: 'creditCard',
        pattern: /\b(?:\d{4}[\s-]?){3}\d{4}\b/g,
        replacement: '[CREDIT_CARD_MASKED]'
      },
      {
        name: 'ssn',
        pattern: /\b\d{3}-\d{2}-\d{4}\b/g,
        replacement: '[SSN_MASKED]'
      },
      {
        name: 'phoneNumber',
        pattern: /(\+?1?\s?)?\(?\d{3}\)?[\s.-]?\d{3}[\s.-]?\d{4}/g,
        replacement: '[PHONE_MASKED]'
      },
      {
        name: 'awsAccessKey',
        pattern: /AKIA[0-9A-Z]{16}/g,
        replacement: '[AWS_ACCESS_KEY_MASKED]'
      },
      {
        name: 'awsSecretKey',
        pattern: /[a-zA-Z0-9\/+=]{40}/g,
        replacement: '[AWS_SECRET_KEY_MASKED]'
      },
      {
        name: 'githubToken',
        pattern: /ghp_[a-zA-Z0-9]{36}/g,
        replacement: '[GITHUB_TOKEN_MASKED]'
      },
      {
        name: 'jwtToken',
        pattern: /eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+/g,
        replacement: '[JWT_TOKEN_MASKED]'
      },
      {
        name: 'databaseUrl',
        pattern: /(?:postgresql|mysql|mongodb|redis):\/\/[^:\s]+:[^@\s]+@[^:\s]+:\d+\/[^\s]+/g,
        replacement: '[DATABASE_URL_MASKED]'
      },
      {
        name: 'redisUrl',
        pattern: /redis:\/\/[^:\s]+:[^@\s]+@[^:\s]+:\d+/g,
        replacement: '[REDIS_URL_MASKED]'
      },
      {
        name: 'webhookUrl',
        pattern: /(?:webhook[_-]?url|callback[_-]?url)["']?\s*[:=]\s*["']?https?:\/\/[^\s"']+["']?/gi,
        replacement: '[WEBHOOK_URL_MASKED]'
      },
      {
        name: 'stripeKey',
        pattern: /(?:sk_live_|pk_live_|sk_test_|pk_test_)[a-zA-Z0-9]{24}/g,
        replacement: '[STRIPE_KEY_MASKED]'
      },
      {
        name: 'sendgridKey',
        pattern: /SG\.[a-zA-Z0-9\-_]{22}\.[a-zA-Z0-9\-_]{43}/g,
        replacement: '[SENDGRID_KEY_MASKED]'
      },
      {
        name: 'twilioApiKey',
        pattern: /AC[a-z0-9]{32}|SK[a-z0-9]{32}/g,
        replacement: '[TWILIO_API_KEY_MASKED]'
      },
      {
        name: 'slackToken',
        pattern: /xox[bpoa]-[0-9]{12}-[0-9]{12}-[0-9]{12}-[a-z0-9]{32}/g,
        replacement: '[SLACK_TOKEN_MASKED]'
      },
      {
        name: 'discordToken',
        pattern: /[MN][A-Za-z\d]{23}\.[\w-]{6}\.[\w-]{27}/g,
        replacement: '[DISCORD_TOKEN_MASKED]'
      },
      {
        name: 'herokuApiKey',
        pattern: /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi,
        replacement: '[HEROKU_API_KEY_MASKED]'
      },
      {
        name: 'dockerhubToken',
        pattern: /(?:dockerhub[_-]?token|docker[_-]?token)["']?\s*[:=]\s*["']?[a-zA-Z0-9\-_]{36}["']?/gi,
        replacement: '[DOCKERHUB_TOKEN_MASKED]'
      },
      {
        name: 'npmToken',
        pattern: /npm_[a-zA-Z0-9]{36}/g,
        replacement: '[NPM_TOKEN_MASKED]'
      },
      {
        name: 'pypiToken',
        pattern: /pypi-AgEIcHlwaS5vcmc[A-Za-z0-9\-_]{32,}/g,
        replacement: '[PYPI_TOKEN_MASKED]'
      },
      {
        name: 'cloudflareToken',
        pattern: /[a-zA-Z0-9_-]{40}/g,
        replacement: '[CLOUDFLARE_TOKEN_MASKED]'
      },
      {
        name: 'mailgunKey',
        pattern: /key-[a-z0-9]{32}/g,
        replacement: '[MAILGUN_KEY_MASKED]'
      },
      {
        name: 'firebaseKey',
        pattern: /AIza[0-9A-Za-z\-_]{35}/g,
        replacement: '[FIREBASE_KEY_MASKED]'
      },
      {
        name: 'awsSessionToken',
        pattern: /(?:aws[_-]?session[_-]?token)["']?\s*[:=]\s*["']?[A-Za-z0-9+\/=]{100,}["']?/gi,
        replacement: '[AWS_SESSION_TOKEN_MASKED]'
      },
      {
        name: 'azureSubscriptionId',
        pattern: /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi,
        replacement: '[AZURE_SUBSCRIPTION_ID_MASKED]'
      },
      {
        name: 'gcpProjectNumber',
        pattern: /(?:project[_-]?number)["']?\s*[:=]\s*["']?\d{12}["']?/gi,
        replacement: '[GCP_PROJECT_NUMBER_MASKED]'
      },
      {
        name: 'mongodbConnectionString',
        pattern: /mongodb(?:\+srv)?:\/\/[^:\s]+:[^@\s]+@[^\/\s]+\/[^\s?]+(?:\?[^\s]*)?/g,
        replacement: '[MONGODB_CONNECTION_MASKED]'
      },
      {
        name: 'elasticsearchUrl',
        pattern: /https?:\/\/[^:\s]+:[^@\s]+@[^:\s]+:\d+/g,
        replacement: '[ELASTICSEARCH_URL_MASKED]'
      },
      {
        name: 'kubeconfigToken',
        pattern: /(?:token|bearer)["']?\s*[:=]\s*["']?[A-Za-z0-9\-_\.]{100,}["']?/gi,
        replacement: '[KUBECONFIG_TOKEN_MASKED]'
      },
      {
        name: 'dockerRegistryAuth',
        pattern: /"auth":\s*"[A-Za-z0-9+\/=]{20,}"/g,
        replacement: '"auth": "[DOCKER_REGISTRY_AUTH_MASKED]"'
      },
      {
        name: 'sshPrivateKey',
        pattern: /-----BEGIN (?:RSA |OPENSSH |DSA |EC |PGP )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA |OPENSSH |DSA |EC |PGP )?PRIVATE KEY-----/g,
        replacement: '[SSH_PRIVATE_KEY_MASKED]'
      },
      {
        name: 'pgpPrivateKey',
        pattern: /-----BEGIN PGP PRIVATE KEY BLOCK-----[\s\S]*?-----END PGP PRIVATE KEY BLOCK-----/g,
        replacement: '[PGP_PRIVATE_KEY_MASKED]'
      }
    ];
  }

  public mask(text: string): [string, Array<{entityType: string, masked: string, count: number}>] {
    let maskedText = text;
    const findings: Array<{entityType: string, masked: string, count: number}> = [];

    for (const rule of this.rules) {
      const matches = maskedText.match(rule.pattern);
      if (matches && matches.length > 0) {
        // Get the replacement value for this rule
        let replacementValue: string;
        if (typeof rule.replacement === 'function') {
          // For function replacements, we need to get a sample replacement
          replacementValue = matches[0].replace(rule.pattern, rule.replacement);
        } else {
          replacementValue = rule.replacement;
        }
        
        // Count unique matches for this rule
        const uniqueMatches = [...new Set(matches)];
        
        findings.push({
          entityType: rule.name,
          masked: replacementValue,
          count: uniqueMatches.length
        });
        
        // Apply the masking
        if (typeof rule.replacement === 'function') {
          maskedText = maskedText.replace(rule.pattern, rule.replacement);
        } else {
          maskedText = maskedText.replace(rule.pattern, rule.replacement);
        }
      }
    }

    return [maskedText, findings];
  }

  public addRule(rule: MaskingRule): void {
    this.rules.push(rule);
  }

  public removeRule(name: string): void {
    this.rules = this.rules.filter(rule => rule.name !== name);
  }

  public getRules(): MaskingRule[] {
    return [...this.rules];
  }
}