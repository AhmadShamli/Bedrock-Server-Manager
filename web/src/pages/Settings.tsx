import React, { useState, useEffect, useRef } from 'react';
import {
  Settings as SettingsIcon, Save, Check, Loader2, AlertCircle, Clock,
  ToggleLeft, ToggleRight, Box, ShieldCheck, Cpu, HardDrive, Download,
  RefreshCw, ExternalLink, Trash2, Shield, Activity, Bell, Send, Mail,
  Server, Database, ShieldAlert, Package, Key, Eye, EyeOff, Globe,
  ChevronDown, ChevronUp
} from 'lucide-react';
import { api } from '../api/client';
import { User, BDSStatus, BDSVersionsResponse, SystemInfo, DDNSStatus } from '../types';
import { DurationUnit, durationToSeconds, secondsToValueAndUnit } from '../utils/duration';
import { TurnstileWidget, TurnstileWidgetRef } from '../components/TurnstileWidget';

interface SettingsPageProps {
  currentUser?: User;
}

export const SettingsPage: React.FC<SettingsPageProps> = () => {
  const [loading, setLoading] = useState(true);
  const [savingSettings, setSavingSettings] = useState(false);
  const [settingsSaved, setSettingsSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // System Diagnostics State
  const [systemInfo, setSystemInfo] = useState<SystemInfo | null>(null);

  // Form State: General & Access Policies
  const [allowRegistration, setAllowRegistration] = useState(false);
  const [defaultGameServerAddress, setDefaultGameServerAddress] = useState('');
  const [heartbeatSec, setHeartbeatSec] = useState(10);

  // Form State: Dynamic DNS (DDNS) & Public IP
  const [ddnsEnabled, setDdnsEnabled] = useState(false);
  const [ddnsProvider, setDdnsProvider] = useState<'cloudflare' | 'duckdns' | 'noip' | 'webhook'>('cloudflare');
  const [ddnsDomain, setDdnsDomain] = useState('');
  const [ddnsEnableIPv4, setDdnsEnableIPv4] = useState(true);
  const [ddnsEnableIPv6, setDdnsEnableIPv6] = useState(false);
  const [ddnsIntervalMinutes, setDdnsIntervalMinutes] = useState(5);
  const [ddnsAutoSyncGameAddress, setDdnsAutoSyncGameAddress] = useState(true);
  const [ddnsAuthToken, setDdnsAuthToken] = useState('');
  const [ddnsZoneId, setDdnsZoneId] = useState('');
  const [ddnsUsername, setDdnsUsername] = useState('');
  const [ddnsPassword, setDdnsPassword] = useState('');
  const [ddnsWebhookUrl, setDdnsWebhookUrl] = useState('');
  const [ddnsWebhookMethod, setDdnsWebhookMethod] = useState<'GET' | 'POST'>('GET');
  const [showDdnsToken, setShowDdnsToken] = useState(false);
  const [showDdnsPassword, setShowDdnsPassword] = useState(false);
  const [ddnsConfigured, setDdnsConfigured] = useState(false);

  // DDNS Live Diagnostics & Actions
  const [ddnsStatus, setDdnsStatus] = useState<DDNSStatus | null>(null);
  const [syncingDDNS, setSyncingDDNS] = useState(false);
  const [testingDDNS, setTestingDDNS] = useState(false);
  const [ddnsMessage, setDdnsMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);
  const [ddnsDetailsOpen, setDdnsDetailsOpen] = useState<boolean | null>(null);

  // Form State: CurseForge Marketplace Integration
  const [curseforgeApiKey, setCurseforgeApiKey] = useState('');
  const [curseForgeConfigured, setCurseForgeConfigured] = useState(false);
  const [showCfKey, setShowCfKey] = useState(false);
  const [savingCfKey, setSavingCfKey] = useState(false);
  const [cfMessage, setCfMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  // Form State: Port Gate Firewall Defaults
  const [leaseVal, setLeaseVal] = useState(120);
  const [leaseUnit, setLeaseUnit] = useState<DurationUnit>('minutes');

  // Form State: Brute-Force & Security Protection
  const [rateLimitMaxAttempts, setRateLimitMaxAttempts] = useState(5);
  const [rateLimitLockoutSec, setRateLimitLockoutSec] = useState(300);
  const [circuitBreakerThreshold, setCircuitBreakerThreshold] = useState(10);
  const [circuitBreakerLockoutMin, setCircuitBreakerLockoutMin] = useState(15);
  const [securityAlertDiscord, setSecurityAlertDiscord] = useState(true);

  // Form State: Cloudflare Turnstile Bot Protection
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileSecretKey, setTurnstileSecretKey] = useState('');
  const [turnstileConfigured, setTurnstileConfigured] = useState(false);
  const [showTurnstileSecret, setShowTurnstileSecret] = useState(false);
  const [turnstileLoginEnabled, setTurnstileLoginEnabled] = useState(true);
  const [turnstileRegisterEnabled, setTurnstileRegisterEnabled] = useState(true);
  const [turnstileSetupEnabled, setTurnstileSetupEnabled] = useState(true);
  const [turnstileKnockEnabled, setTurnstileKnockEnabled] = useState(true);
  const [turnstileFailOpen, setTurnstileFailOpen] = useState(false);

  // Turnstile Interactive Testing State
  const [testingTurnstile, setTestingTurnstile] = useState(false);
  const [turnstileTestToken, setTurnstileTestToken] = useState('');
  const [turnstileTestMessage, setTurnstileTestMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);
  const [turnstileDetailsOpen, setTurnstileDetailsOpen] = useState<boolean | null>(null);
  const turnstileTestRef = useRef<TurnstileWidgetRef>(null);

  // Computed collapse state: default to enabled status unless user explicitly toggled details
  const isDDNSOpen = ddnsDetailsOpen !== null ? ddnsDetailsOpen : ddnsEnabled;
  const isTurnstileOpen = turnstileDetailsOpen !== null ? turnstileDetailsOpen : turnstileEnabled;

  // Form State: Resource Monitoring Alerts
  const [alertThreshold, setAlertThreshold] = useState(85);
  const [alertIntervalSec, setAlertIntervalSec] = useState(60);

  // Form State: Discord Webhook
  const [discordWebhookUrl, setDiscordWebhookUrl] = useState('');
  const [discordConfigured, setDiscordConfigured] = useState(false);
  const [testingWebhook, setTestingWebhook] = useState(false);
  const [webhookTestMessage, setWebhookTestMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  // Form State: SMTP Email Delivery
  const [smtpEnabled, setSmtpEnabled] = useState(false);
  const [smtpHost, setSmtpHost] = useState('');
  const [smtpPort, setSmtpPort] = useState(587);
  const [smtpEncryption, setSmtpEncryption] = useState('starttls');
  const [smtpAuthType, setSmtpAuthType] = useState('plain');
  const [smtpUsername, setSmtpUsername] = useState('');
  const [smtpPassword, setSmtpPassword] = useState('');
  const [smtpPasswordConfigured, setSmtpPasswordConfigured] = useState(false);
  const [smtpFromEmail, setSmtpFromEmail] = useState('');
  const [smtpFromName, setSmtpFromName] = useState('Bedrock Server Manager');
  const [smtpSkipTlsVerify, setSmtpSkipTlsVerify] = useState(false);
  const [appUrl, setAppUrl] = useState('');
  const [smtpRequireEmailVerification, setSmtpRequireEmailVerification] = useState(false);
  const [smtpSendWelcomeEmail, setSmtpSendWelcomeEmail] = useState(false);
  const [smtpAlertRecipients, setSmtpAlertRecipients] = useState('');
  const [smtpAlertCrashes, setSmtpAlertCrashes] = useState(false);
  const [smtpAlertSecurity, setSmtpAlertSecurity] = useState(false);
  const [smtpAlertResources, setSmtpAlertResources] = useState(false);
  const [smtpTestRecipient, setSmtpTestRecipient] = useState('');
  const [testingSMTP, setTestingSMTP] = useState(false);
  const [smtpTestMessage, setSmtpTestMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);
  const [showSmtpPassword, setShowSmtpPassword] = useState(false);
  const [smtpDetailsOpen, setSmtpDetailsOpen] = useState<boolean | null>(null);
  const isSmtpOpen = smtpDetailsOpen !== null ? smtpDetailsOpen : smtpEnabled;

  // Form State: Backup Retention Defaults
  const [backupRetentionMaxCount, setBackupRetentionMaxCount] = useState(10);
  const [backupRetentionMaxDays, setBackupRetentionMaxDays] = useState(14);
  const [backupRetentionMaxGB, setBackupRetentionMaxGB] = useState(5);

  // BDS & Windows Host State
  const [bdsStatus, setBdsStatus] = useState<BDSStatus | null>(null);
  const [bdsVersions, setBdsVersions] = useState<BDSVersionsResponse | null>(null);
  const [bdsActionLoading, setBdsActionLoading] = useState<string | null>(null);
  const [bdsMessage, setBdsMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [settingsRes, bdsStatusRes, sysInfoRes, marketplaceRes, ddnsStatusRes, ddnsConfigRes] = await Promise.all([
        api.getSettings().catch((): Record<string, string> => ({})),
        api.getBDSStatus().catch(() => null),
        api.getSystemInfo().catch(() => null),
        api.getMarketplaceConfig().catch(() => null),
        api.getDDNSStatus().catch(() => null),
        api.getDDNSConfig().catch(() => null),
      ]);

      if (sysInfoRes) {
        setSystemInfo(sysInfoRes);
      }
      if (bdsStatusRes) {
        setBdsStatus(bdsStatusRes);
      }
      if (marketplaceRes) {
        setCurseForgeConfigured(!!marketplaceRes.curseforge_configured);
      } else if (settingsRes['curseforge_api_key']) {
        setCurseForgeConfigured(true);
      }

      if (ddnsStatusRes) {
        setDdnsStatus(ddnsStatusRes);
      }
      if (ddnsConfigRes) {
        setDdnsEnabled(ddnsConfigRes.enabled);
        setDdnsProvider(ddnsConfigRes.provider || 'cloudflare');
        setDdnsDomain(ddnsConfigRes.domain || '');
        setDdnsEnableIPv4(ddnsConfigRes.enable_ipv4 !== false);
        setDdnsEnableIPv6(!!ddnsConfigRes.enable_ipv6);
        setDdnsIntervalMinutes(ddnsConfigRes.interval_minutes || 5);
        setDdnsAutoSyncGameAddress(ddnsConfigRes.auto_sync_game_address !== false);
        setDdnsZoneId(ddnsConfigRes.zone_id || '');
        setDdnsUsername(ddnsConfigRes.username || '');
        setDdnsWebhookUrl(ddnsConfigRes.webhook_url || '');
        setDdnsWebhookMethod(ddnsConfigRes.webhook_method || 'GET');
        if (ddnsConfigRes.auth_token) setDdnsConfigured(true);
      }

      // Populate General & Access
      if (settingsRes['allow_registration']) {
        setAllowRegistration(settingsRes['allow_registration'] === 'true' || settingsRes['allow_registration'] === '1');
      }
      if (settingsRes['default_game_server_address']) {
        setDefaultGameServerAddress(settingsRes['default_game_server_address']);
      }
      if (settingsRes['heartbeat_interval_seconds']) {
        const val = parseInt(settingsRes['heartbeat_interval_seconds'], 10);
        if (!isNaN(val) && val > 0) setHeartbeatSec(val);
      }

      // Populate Port Gate Defaults
      if (settingsRes['default_portgate_lease_seconds']) {
        const totalSec = parseInt(settingsRes['default_portgate_lease_seconds'], 10);
        if (!isNaN(totalSec) && totalSec > 0) {
          const parsed = secondsToValueAndUnit(totalSec);
          setLeaseVal(parsed.value);
          setLeaseUnit(parsed.unit);
        }
      }

      // Populate Brute-Force & Security Protection
      if (settingsRes['rate_limit_max_attempts']) {
        const val = parseInt(settingsRes['rate_limit_max_attempts'], 10);
        if (!isNaN(val) && val > 0) setRateLimitMaxAttempts(val);
      }
      if (settingsRes['rate_limit_lockout_seconds']) {
        const val = parseInt(settingsRes['rate_limit_lockout_seconds'], 10);
        if (!isNaN(val) && val > 0) setRateLimitLockoutSec(val);
      }
      if (settingsRes['circuit_breaker_threshold']) {
        const val = parseInt(settingsRes['circuit_breaker_threshold'], 10);
        if (!isNaN(val) && val > 0) setCircuitBreakerThreshold(val);
      }
      if (settingsRes['circuit_breaker_lockout_minutes']) {
        const val = parseInt(settingsRes['circuit_breaker_lockout_minutes'], 10);
        if (!isNaN(val) && val > 0) setCircuitBreakerLockoutMin(val);
      }
      if (settingsRes['security_alert_discord'] !== undefined) {
        setSecurityAlertDiscord(settingsRes['security_alert_discord'] === 'true' || settingsRes['security_alert_discord'] === '1');
      }

      // Populate Turnstile Settings
      if (settingsRes['turnstile_enabled'] !== undefined) {
        setTurnstileEnabled(settingsRes['turnstile_enabled'] === 'true' || settingsRes['turnstile_enabled'] === '1');
      }
      if (settingsRes['turnstile_site_key']) {
        setTurnstileSiteKey(settingsRes['turnstile_site_key']);
      }
      if (settingsRes['turnstile_secret_key']) {
        setTurnstileConfigured(true);
      }
      if (settingsRes['turnstile_login_enabled'] !== undefined) {
        setTurnstileLoginEnabled(settingsRes['turnstile_login_enabled'] === 'true' || settingsRes['turnstile_login_enabled'] === '1');
      }
      if (settingsRes['turnstile_register_enabled'] !== undefined) {
        setTurnstileRegisterEnabled(settingsRes['turnstile_register_enabled'] === 'true' || settingsRes['turnstile_register_enabled'] === '1');
      }
      if (settingsRes['turnstile_setup_enabled'] !== undefined) {
        setTurnstileSetupEnabled(settingsRes['turnstile_setup_enabled'] === 'true' || settingsRes['turnstile_setup_enabled'] === '1');
      }
      if (settingsRes['turnstile_knock_enabled'] !== undefined) {
        setTurnstileKnockEnabled(settingsRes['turnstile_knock_enabled'] === 'true' || settingsRes['turnstile_knock_enabled'] === '1');
      }
      if (settingsRes['turnstile_fail_open'] !== undefined) {
        setTurnstileFailOpen(settingsRes['turnstile_fail_open'] === 'true' || settingsRes['turnstile_fail_open'] === '1');
      }

      // Populate Resource Alerts
      if (settingsRes['resource_alert_threshold']) {
        const val = parseFloat(settingsRes['resource_alert_threshold']);
        if (!isNaN(val) && val > 0) setAlertThreshold(Math.round(val <= 1 ? val * 100 : val));
      }
      if (settingsRes['resource_alert_interval_seconds']) {
        const val = parseInt(settingsRes['resource_alert_interval_seconds'], 10);
        if (!isNaN(val) && val > 0) setAlertIntervalSec(val);
      }

      // Populate Discord Webhook
      if (settingsRes['discord_webhook_url']) {
        setDiscordConfigured(true);
      }

      // Populate SMTP Settings
      if (settingsRes['smtp_enabled'] !== undefined) {
        setSmtpEnabled(settingsRes['smtp_enabled'] === 'true' || settingsRes['smtp_enabled'] === '1');
      }
      if (settingsRes['smtp_host']) {
        setSmtpHost(settingsRes['smtp_host']);
      }
      if (settingsRes['smtp_port']) {
        const val = parseInt(settingsRes['smtp_port'], 10);
        if (!isNaN(val) && val > 0) setSmtpPort(val);
      }
      if (settingsRes['smtp_encryption']) {
        setSmtpEncryption(settingsRes['smtp_encryption']);
      }
      if (settingsRes['smtp_auth_type']) {
        setSmtpAuthType(settingsRes['smtp_auth_type']);
      }
      if (settingsRes['smtp_username']) {
        setSmtpUsername(settingsRes['smtp_username']);
      }
      if (settingsRes['smtp_password']) {
        setSmtpPassword(settingsRes['smtp_password']);
        setSmtpPasswordConfigured(settingsRes['smtp_password'] === '******' || settingsRes['smtp_password'].length > 0);
      }
      if (settingsRes['smtp_from_email']) {
        setSmtpFromEmail(settingsRes['smtp_from_email']);
      }
      if (settingsRes['smtp_from_name']) {
        setSmtpFromName(settingsRes['smtp_from_name']);
      }
      if (settingsRes['smtp_skip_tls_verify'] !== undefined) {
        setSmtpSkipTlsVerify(settingsRes['smtp_skip_tls_verify'] === 'true' || settingsRes['smtp_skip_tls_verify'] === '1');
      }
      if (settingsRes['app_url']) {
        setAppUrl(settingsRes['app_url']);
      }
      if (settingsRes['smtp_require_email_verification'] !== undefined) {
        setSmtpRequireEmailVerification(settingsRes['smtp_require_email_verification'] === 'true' || settingsRes['smtp_require_email_verification'] === '1');
      }
      if (settingsRes['smtp_send_welcome_email'] !== undefined) {
        setSmtpSendWelcomeEmail(settingsRes['smtp_send_welcome_email'] === 'true' || settingsRes['smtp_send_welcome_email'] === '1');
      }
      if (settingsRes['smtp_alert_recipients'] !== undefined) {
        setSmtpAlertRecipients(settingsRes['smtp_alert_recipients']);
      }
      if (settingsRes['smtp_alert_crashes'] !== undefined) {
        setSmtpAlertCrashes(settingsRes['smtp_alert_crashes'] === 'true' || settingsRes['smtp_alert_crashes'] === '1');
      }
      if (settingsRes['smtp_alert_security'] !== undefined) {
        setSmtpAlertSecurity(settingsRes['smtp_alert_security'] === 'true' || settingsRes['smtp_alert_security'] === '1');
      }
      if (settingsRes['smtp_alert_resources'] !== undefined) {
        setSmtpAlertResources(settingsRes['smtp_alert_resources'] === 'true' || settingsRes['smtp_alert_resources'] === '1');
      }

      // Populate Backup Retention
      if (settingsRes['backup_retention_max_count']) {
        const val = parseInt(settingsRes['backup_retention_max_count'], 10);
        if (!isNaN(val) && val > 0) setBackupRetentionMaxCount(val);
      }
      if (settingsRes['backup_retention_max_days']) {
        const val = parseInt(settingsRes['backup_retention_max_days'], 10);
        if (!isNaN(val) && val > 0) setBackupRetentionMaxDays(val);
      }
      if (settingsRes['backup_retention_max_gb']) {
        const val = parseFloat(settingsRes['backup_retention_max_gb']);
        if (!isNaN(val) && val > 0) setBackupRetentionMaxGB(val);
      }
    } catch (err: any) {
      setError(err.message || 'Failed to load system settings');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleSaveAllSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingSettings(true);
    setSettingsSaved(false);
    setError(null);

    try {
      const leaseSeconds = durationToSeconds(leaseVal, leaseUnit);

      const updates: Promise<any>[] = [
        api.updateSetting('allow_registration', allowRegistration ? 'true' : 'false'),
        api.updateSetting('default_game_server_address', defaultGameServerAddress.trim()),
        api.updateSetting('heartbeat_interval_seconds', heartbeatSec.toString()),
        api.updateSetting('default_portgate_lease_seconds', leaseSeconds.toString()),
        api.updateSetting('rate_limit_max_attempts', rateLimitMaxAttempts.toString()),
        api.updateSetting('rate_limit_lockout_seconds', rateLimitLockoutSec.toString()),
        api.updateSetting('circuit_breaker_threshold', circuitBreakerThreshold.toString()),
        api.updateSetting('circuit_breaker_lockout_minutes', circuitBreakerLockoutMin.toString()),
        api.updateSetting('security_alert_discord', securityAlertDiscord ? 'true' : 'false'),
        api.updateSetting('resource_alert_threshold', alertThreshold.toString()),
        api.updateSetting('resource_alert_interval_seconds', alertIntervalSec.toString()),
        api.updateSetting('backup_retention_max_count', backupRetentionMaxCount.toString()),
        api.updateSetting('backup_retention_max_days', backupRetentionMaxDays.toString()),
        api.updateSetting('backup_retention_max_gb', backupRetentionMaxGB.toString()),
      ];

      // Turnstile validation & updates
      if (turnstileEnabled) {
        if (!turnstileSiteKey.trim()) {
          throw new Error('Cloudflare Turnstile cannot be enabled without a valid Site Key.');
        }
        if (!turnstileSecretKey.trim() && !turnstileConfigured) {
          throw new Error('Cloudflare Turnstile cannot be enabled without a valid Secret Key.');
        }
      }

      updates.push(
        api.updateSetting('turnstile_enabled', turnstileEnabled ? 'true' : 'false'),
        api.updateSetting('turnstile_site_key', turnstileSiteKey.trim()),
        api.updateSetting('turnstile_login_enabled', turnstileLoginEnabled ? 'true' : 'false'),
        api.updateSetting('turnstile_register_enabled', turnstileRegisterEnabled ? 'true' : 'false'),
        api.updateSetting('turnstile_setup_enabled', turnstileSetupEnabled ? 'true' : 'false'),
        api.updateSetting('turnstile_knock_enabled', turnstileKnockEnabled ? 'true' : 'false'),
        api.updateSetting('turnstile_fail_open', turnstileFailOpen ? 'true' : 'false')
      );

      if (turnstileSecretKey.trim() && turnstileSecretKey !== '******') {
        updates.push(api.updateSetting('turnstile_secret_key', turnstileSecretKey.trim()));
      }

      if (curseforgeApiKey.trim()) {
        updates.push(api.updateMarketplaceConfig(curseforgeApiKey.trim()));
      }

      if (discordWebhookUrl.trim()) {
        updates.push(api.updateSetting('discord_webhook_url', discordWebhookUrl.trim()));
      }

      updates.push(
        api.updateSetting('smtp_enabled', smtpEnabled ? 'true' : 'false'),
        api.updateSetting('smtp_host', smtpHost.trim()),
        api.updateSetting('smtp_port', smtpPort.toString()),
        api.updateSetting('smtp_encryption', smtpEncryption),
        api.updateSetting('smtp_auth_type', smtpAuthType),
        api.updateSetting('smtp_username', smtpUsername.trim()),
        api.updateSetting('smtp_from_email', smtpFromEmail.trim()),
        api.updateSetting('smtp_from_name', smtpFromName.trim()),
        api.updateSetting('smtp_skip_tls_verify', smtpSkipTlsVerify ? 'true' : 'false'),
        api.updateSetting('app_url', appUrl.trim()),
        api.updateSetting('smtp_require_email_verification', smtpRequireEmailVerification ? 'true' : 'false'),
        api.updateSetting('smtp_send_welcome_email', smtpSendWelcomeEmail ? 'true' : 'false'),
        api.updateSetting('smtp_alert_recipients', smtpAlertRecipients.trim()),
        api.updateSetting('smtp_alert_crashes', smtpAlertCrashes ? 'true' : 'false'),
        api.updateSetting('smtp_alert_security', smtpAlertSecurity ? 'true' : 'false'),
        api.updateSetting('smtp_alert_resources', smtpAlertResources ? 'true' : 'false')
      );

      if (smtpPassword.trim() && smtpPassword !== '******') {
        updates.push(api.updateSetting('smtp_password', smtpPassword.trim()));
      }

      updates.push(
        api.updateDDNSConfig({
          enabled: ddnsEnabled,
          provider: ddnsProvider,
          domain: ddnsDomain.trim(),
          enable_ipv4: ddnsEnableIPv4,
          enable_ipv6: ddnsEnableIPv6,
          interval_minutes: ddnsIntervalMinutes,
          auto_sync_game_address: ddnsAutoSyncGameAddress,
          auth_token: ddnsAuthToken.trim() || undefined,
          zone_id: ddnsZoneId.trim() || undefined,
          username: ddnsUsername.trim() || undefined,
          password: ddnsPassword.trim() || undefined,
          webhook_url: ddnsWebhookUrl.trim() || undefined,
          webhook_method: ddnsWebhookMethod,
        })
      );

      await Promise.all(updates);

      if (smtpPassword.trim() && smtpPassword !== '******') {
        setSmtpPasswordConfigured(true);
        setSmtpPassword('******');
      }

      if (turnstileSecretKey.trim()) {
        setTurnstileConfigured(true);
        setTurnstileSecretKey('');
      }

      if (curseforgeApiKey.trim()) {
        setCurseForgeConfigured(true);
        setCurseforgeApiKey('');
      }
      if (discordWebhookUrl.trim()) {
        setDiscordConfigured(true);
        setDiscordWebhookUrl('');
      }

      if (ddnsAuthToken.trim()) {
        setDdnsConfigured(true);
        setDdnsAuthToken('');
      }
      if (ddnsPassword.trim()) {
        setDdnsPassword('');
      }
      api.getDDNSStatus().then((st) => setDdnsStatus(st)).catch(() => null);

      setSettingsSaved(true);
      setTimeout(() => setSettingsSaved(false), 4000);
    } catch (err: any) {
      setError(err.message || 'Failed to update system settings');
    } finally {
      setSavingSettings(false);
    }
  };

  const handleSyncDDNS = async () => {
    setSyncingDDNS(true);
    setDdnsMessage(null);
    try {
      const res = await api.syncDDNS();
      if (res.success && res.result) {
        setDdnsMessage({
          text: `Success: ${res.result.message || 'DNS record synchronized successfully'}`,
          type: 'success',
        });
        if (res.status) {
          setDdnsStatus(res.status);
        }
      } else {
        setDdnsMessage({
          text: res.error || 'Failed to synchronize DDNS record',
          type: 'error',
        });
      }
    } catch (err: any) {
      setDdnsMessage({
        text: err.message || 'Failed to trigger DDNS sync',
        type: 'error',
      });
    } finally {
      setSyncingDDNS(false);
    }
  };

  const handleTestDDNS = async () => {
    setTestingDDNS(true);
    setDdnsMessage(null);
    try {
      const res = await api.testDDNS({
        enabled: ddnsEnabled,
        provider: ddnsProvider,
        domain: ddnsDomain.trim(),
        enable_ipv4: ddnsEnableIPv4,
        enable_ipv6: ddnsEnableIPv6,
        interval_minutes: ddnsIntervalMinutes,
        auto_sync_game_address: ddnsAutoSyncGameAddress,
        auth_token: ddnsAuthToken.trim() || undefined,
        zone_id: ddnsZoneId.trim() || undefined,
        username: ddnsUsername.trim() || undefined,
        password: ddnsPassword.trim() || undefined,
        webhook_url: ddnsWebhookUrl.trim() || undefined,
        webhook_method: ddnsWebhookMethod,
      });
      if (res.success) {
        setDdnsMessage({
          text: `Verification passed: ${res.result?.message || 'Credentials and connectivity verified'}`,
          type: 'success',
        });
      } else {
        setDdnsMessage({
          text: `Test failed: ${res.error || 'Provider rejected credentials'}`,
          type: 'error',
        });
      }
    } catch (err: any) {
      setDdnsMessage({
        text: err.message || 'DDNS connection test failed',
        type: 'error',
      });
    } finally {
      setTestingDDNS(false);
    }
  };

  const handleTestTurnstile = async () => {
    if (!turnstileTestToken) {
      setTurnstileTestMessage({ text: 'Please complete the Turnstile challenge widget below first.', type: 'error' });
      return;
    }
    setTestingTurnstile(true);
    setTurnstileTestMessage(null);
    try {
      const res = await api.testTurnstile({
        site_key: turnstileSiteKey.trim() || undefined,
        secret_key: turnstileSecretKey.trim() || undefined,
        token: turnstileTestToken,
      });
      if (res.success) {
        setTurnstileTestMessage({
          text: `Success! Cloudflare Turnstile verification passed (Challenge TS: ${res.challenge_ts || 'OK'}).`,
          type: 'success',
        });
      } else {
        const errCodes = res['error-codes']?.join(', ') || 'invalid challenge response';
        setTurnstileTestMessage({
          text: `Verification failed from Cloudflare: ${errCodes}`,
          type: 'error',
        });
      }
    } catch (err: any) {
      setTurnstileTestMessage({
        text: err.message || 'Turnstile test request failed',
        type: 'error',
      });
    } finally {
      setTestingTurnstile(false);
      setTurnstileTestToken('');
      turnstileTestRef.current?.reset();
    }
  };

  const handleSaveCurseForgeKey = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!curseforgeApiKey.trim()) return;
    setSavingCfKey(true);
    setCfMessage(null);
    try {
      const res = await api.updateMarketplaceConfig(curseforgeApiKey.trim());
      setCurseForgeConfigured(!!res.curseforge_configured);
      setCurseforgeApiKey('');
      setCfMessage({ text: 'CurseForge API key saved successfully.', type: 'success' });
    } catch (err: any) {
      setCfMessage({ text: err.message || 'Failed to save CurseForge API key', type: 'error' });
    } finally {
      setSavingCfKey(false);
    }
  };

  const handleClearCurseForgeKey = async () => {
    if (!window.confirm('Are you sure you want to remove the configured CurseForge API key? Community marketplace addon downloads will be disabled.')) return;
    setSavingCfKey(true);
    setCfMessage(null);
    try {
      const res = await api.updateMarketplaceConfig('');
      setCurseForgeConfigured(!!res.curseforge_configured);
      setCurseforgeApiKey('');
      setCfMessage({ text: 'CurseForge API key removed successfully.', type: 'success' });
    } catch (err: any) {
      setCfMessage({ text: err.message || 'Failed to remove CurseForge API key', type: 'error' });
    } finally {
      setSavingCfKey(false);
    }
  };

  const handleTestDiscordWebhook = async () => {
    setTestingWebhook(true);
    setWebhookTestMessage(null);
    try {
      const res = await api.testDiscordWebhook(discordWebhookUrl.trim() || undefined);
      setWebhookTestMessage({
        text: res.message || 'Test notification sent to Discord channel successfully!',
        type: 'success',
      });
    } catch (err: any) {
      setWebhookTestMessage({
        text: err.message || 'Failed to send Discord test webhook. Verify URL.',
        type: 'error',
      });
    } finally {
      setTestingWebhook(false);
    }
  };

  const handleTestSMTP = async () => {
    setTestingSMTP(true);
    setSmtpTestMessage(null);
    try {
      const res = await api.testSMTP({
        recipient: smtpTestRecipient.trim() || undefined,
        host: smtpHost.trim() || undefined,
        port: smtpPort || undefined,
        encryption: smtpEncryption || undefined,
        auth_type: smtpAuthType || undefined,
        username: smtpUsername.trim() || undefined,
        password: smtpPassword.trim() && smtpPassword !== '******' ? smtpPassword.trim() : undefined,
        from_email: smtpFromEmail.trim() || undefined,
        from_name: smtpFromName.trim() || undefined,
        skip_tls_verify: smtpSkipTlsVerify,
      });
      setSmtpTestMessage({
        text: res.message || 'Test email dispatched successfully! Please check the recipient inbox.',
        type: 'success',
      });
    } catch (err: any) {
      setSmtpTestMessage({
        text: err.message || 'SMTP test execution failed.',
        type: 'error',
      });
    } finally {
      setTestingSMTP(false);
    }
  };

  const handleAcceptEula = async () => {
    try {
      setBdsActionLoading('eula');
      await api.acceptBDSEULA();
      const st = await api.getBDSStatus();
      setBdsStatus(st);
      setBdsMessage({ text: 'Mojang BDS EULA accepted successfully.', type: 'success' });
    } catch (err: any) {
      setBdsMessage({ text: err.message || 'Failed to accept EULA', type: 'error' });
    } finally {
      setBdsActionLoading(null);
    }
  };

  const handlePurgeCache = async () => {
    if (!confirm('Purge cached BDS versions that are not currently in use by any server?')) return;
    try {
      setBdsActionLoading('purge');
      await api.purgeBDSCache();
      const st = await api.getBDSStatus();
      setBdsStatus(st);
      setBdsMessage({ text: 'BDS cache purged successfully.', type: 'success' });
    } catch (err: any) {
      setBdsMessage({ text: err.message || 'Failed to purge cache', type: 'error' });
    } finally {
      setBdsActionLoading(null);
    }
  };

  const handleCheckVersions = async () => {
    try {
      setBdsActionLoading('versions');
      const v = await api.checkBDSVersions(true);
      setBdsVersions(v);
      setBdsMessage({
        text: `BDS versions checked. Latest: ${v.latest?.version || 'unknown'}, Preview: ${v.preview?.version || 'unknown'}`,
        type: 'success',
      });
    } catch (err: any) {
      setBdsMessage({ text: err.message || 'Failed to check versions', type: 'error' });
    } finally {
      setBdsActionLoading(null);
    }
  };

  const handleSetLoopback = async () => {
    try {
      setBdsActionLoading('loopback');
      const res = await api.setLoopbackExemption();
      setBdsMessage({ text: res.output || 'Loopback exemption enabled successfully.', type: 'success' });
    } catch (err: any) {
      setBdsMessage({ text: err.message || 'Failed to enable loopback exemption', type: 'error' });
    } finally {
      setBdsActionLoading(null);
    }
  };

  const handleAddDefender = async () => {
    try {
      setBdsActionLoading('defender');
      const res = await api.addDefenderExclusion();
      setBdsMessage({
        text: `Windows Defender exclusion added for ${res.exclusion_path || 'data directory'}.`,
        type: 'success',
      });
    } catch (err: any) {
      setBdsMessage({ text: err.message || 'Failed to add Defender exclusion', type: 'error' });
    } finally {
      setBdsActionLoading(null);
    }
  };

  const formatUptime = (seconds?: number) => {
    if (!seconds) return '0s';
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    const parts = [];
    if (d > 0) parts.push(`${d}d`);
    if (h > 0) parts.push(`${h}h`);
    if (m > 0) parts.push(`${m}m`);
    parts.push(`${s}s`);
    return parts.join(' ');
  };

  if (loading) {
    return (
      <div className="min-h-[70vh] flex items-center justify-center text-slate-400">
        <Loader2 className="w-8 h-8 animate-spin text-emerald-400" />
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
      {/* HEADER */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 pb-6 border-b border-obsidian-800">
        <div>
          <h1 className="text-2xl font-mono font-bold text-slate-100 flex items-center gap-3">
            <SettingsIcon className="w-6 h-6 text-emerald-400" />
            <span>System Settings</span>
          </h1>
          <p className="text-sm font-mono text-slate-400 mt-1">
            Global policies, engine configurations, notification webhooks, and host runtime diagnostics.
          </p>
        </div>

        <button
          onClick={handleSaveAllSettings}
          disabled={savingSettings}
          className="px-5 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-2 shadow-[0_0_15px_rgba(16,185,129,0.2)] transition-all disabled:opacity-50"
        >
          {savingSettings ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
          <span>Save All Settings</span>
        </button>
      </div>

      {/* SUCCESS / ERROR ALERTS */}
      {settingsSaved && (
        <div className="p-4 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 font-mono text-xs flex items-center space-x-3 shadow-lg">
          <Check className="w-5 h-5 text-emerald-400 shrink-0" />
          <span className="font-bold">System settings updated and synchronized successfully!</span>
        </div>
      )}

      {error && (
        <div className="p-4 rounded-xl bg-rose-950/40 border border-rose-800 text-rose-300 flex items-center space-x-3 text-sm font-mono">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* 1. SYSTEM & PLATFORM DIAGNOSTICS CARD */}
      {systemInfo && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Activity className="w-4 h-4 text-emerald-400" />
              <h2 className="font-mono text-sm font-bold text-slate-200 uppercase tracking-wider">
                System Diagnostics &amp; Runtime Stack
              </h2>
            </div>
            <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
              Live Daemon Stats
            </span>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 font-mono text-xs">
            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase">Host Platform</div>
              <div className="text-slate-200 font-bold mt-1 flex items-center gap-1.5 truncate">
                <Server className="w-3.5 h-3.5 text-cyan-400 shrink-0" />
                <span>{systemInfo.os} / {systemInfo.arch}</span>
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase">BSM Version</div>
              <div className="text-emerald-400 font-bold mt-1 truncate">
                v{systemInfo.version}
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase">Go Runtime</div>
              <div className="text-slate-300 font-bold mt-1 truncate">
                {systemInfo.go_version}
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase">Goroutines &amp; RAM</div>
              <div className="text-slate-300 font-bold mt-1 truncate">
                {systemInfo.goroutines} gr / {systemInfo.alloc_mb.toFixed(1)} MB
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase">Proxy / IP Mode</div>
              <div className="text-indigo-400 font-bold mt-1 uppercase">
                {systemInfo.proxy_mode || 'direct'}
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase">Firewall Driver</div>
              <div className="text-amber-400 font-bold mt-1 uppercase">
                {systemInfo.firewall_driver || 'ufw'}
              </div>
            </div>
          </div>

          <div className="text-[11px] font-mono text-slate-500 flex items-center justify-between pt-1">
            <span>Daemon Uptime: <strong className="text-slate-300">{formatUptime(systemInfo.uptime_sec)}</strong></span>
            <span>Heap Sys: <strong className="text-slate-300">{systemInfo.sys_mb.toFixed(1)} MB</strong></span>
          </div>
        </div>
      )}

      <form onSubmit={handleSaveAllSettings} className="space-y-8">
        {/* 2. GENERAL SYSTEM & ACCESS POLICIES CARD */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
              <Clock className="w-4 h-4 text-emerald-400" />
              <span>General System &amp; Portal Policies</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono">
              Configure open self-registration, default Knock portal hostnames, and marketplace API keys.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs">
            {/* Registration toggle */}
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="font-bold text-slate-200">Public Self-Registration</span>
                <button
                  type="button"
                  onClick={() => setAllowRegistration(!allowRegistration)}
                  className="text-emerald-400 focus:outline-none"
                >
                  {allowRegistration ? (
                    <ToggleRight className="w-6 h-6 text-emerald-400" />
                  ) : (
                    <ToggleLeft className="w-6 h-6 text-slate-600" />
                  )}
                </button>
              </div>
              <p className="text-[11px] text-slate-400">
                {allowRegistration
                  ? 'Enabled: Visitors can register accounts at /register and automatically receive the Default Plan.'
                  : 'Disabled: New user accounts can only be created by administrators.'}
              </p>
            </div>

            {/* Default Game Server Address */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Default Game Host / FQDN (Optional)
              </label>
              <input
                type="text"
                placeholder="e.g. play.example.com or 203.0.113.10"
                value={defaultGameServerAddress}
                onChange={(e) => setDefaultGameServerAddress(e.target.value)}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Fallback public hostname or IP displayed to players for Knock portal direct-connect links.
              </p>
            </div>

            {/* Heartbeat seconds */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Knock Roaming Heartbeat Interval (seconds)
              </label>
              <input
                type="number"
                min="3"
                max="120"
                value={heartbeatSec}
                onChange={(e) => setHeartbeatSec(parseInt(e.target.value) || 10)}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Client ping interval for mobile client IP renewal during roaming handoffs (default: 10s).
              </p>
            </div>
          </div>
        </div>

        {/* 3. DYNAMIC DNS (DDNS) & PUBLIC IP SYNCHRONIZATION */}
        <div id="ddns" className={`bg-obsidian-900 border border-obsidian-700/80 rounded-xl transition-all ${isDDNSOpen ? 'p-6 space-y-6' : 'p-5'}`}>
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
                <Globe className="w-4 h-4 text-emerald-400" />
                <span>Dynamic DNS (DDNS) &amp; Public IP Synchronization</span>
              </h3>
              <p className="text-xs text-slate-400 font-mono">
                Automatically track home/dynamic public IPs and synchronize DNS records across Cloudflare, DuckDNS, No-IP, or Webhooks.
              </p>
            </div>
            <div className="flex items-center gap-2.5 flex-shrink-0">
              <span className={`px-2.5 py-1 rounded-full text-[10px] font-mono font-bold uppercase tracking-wider border ${
                !ddnsEnabled
                  ? 'bg-slate-800 text-slate-400 border-slate-700'
                  : ddnsStatus?.status === 'success'
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                  : ddnsStatus?.status === 'syncing'
                  ? 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30 animate-pulse'
                  : ddnsStatus?.status === 'error'
                  ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/30'
              }`}>
                {!ddnsEnabled ? 'Disabled' : ddnsStatus?.status || 'Active'}
              </span>

              <label className="inline-flex items-center gap-2 cursor-pointer select-none px-3 py-1.5 rounded-lg bg-obsidian-950 border border-obsidian-800 hover:border-obsidian-700 transition-colors">
                <input
                  type="checkbox"
                  checked={ddnsEnabled}
                  onChange={(e) => {
                    const checked = e.target.checked;
                    setDdnsEnabled(checked);
                    setDdnsDetailsOpen(checked);
                  }}
                  className="w-4 h-4 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                />
                <span className={`text-xs font-mono font-bold ${ddnsEnabled ? 'text-emerald-400' : 'text-slate-400'}`}>
                  {ddnsEnabled ? 'Enabled' : 'Enable'}
                </span>
              </label>

              <button
                type="button"
                onClick={() => setDdnsDetailsOpen(!isDDNSOpen)}
                className="p-1.5 rounded-lg bg-obsidian-950 border border-obsidian-800 hover:border-obsidian-700 text-slate-400 hover:text-slate-200 transition-colors flex items-center gap-1 text-xs font-mono"
                title={isDDNSOpen ? 'Collapse card' : 'Configure settings'}
                aria-label={isDDNSOpen ? 'Collapse Dynamic DNS settings' : 'Configure Dynamic DNS settings'}
              >
                <span className="hidden sm:inline text-[11px]">{isDDNSOpen ? 'Hide' : 'Configure'}</span>
                {isDDNSOpen ? <ChevronUp className="w-4 h-4 text-slate-400" /> : <ChevronDown className="w-4 h-4 text-slate-400" />}
              </button>
            </div>
          </div>

          {isDDNSOpen && (
            <>

          {/* Live Diagnostics & Detected IPs Bar */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 font-mono text-xs">
            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase flex items-center justify-between">
                <span>Detected IPv4</span>
                <span className="text-[9px] text-emerald-400">A Record</span>
              </div>
              <div className="text-slate-200 font-bold mt-1 truncate">
                {ddnsStatus?.current_ipv4 || 'Detecting...'}
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase flex items-center justify-between">
                <span>Detected IPv6</span>
                <span className="text-[9px] text-cyan-400">AAAA Record</span>
              </div>
              <div className="text-slate-300 font-bold mt-1 truncate">
                {ddnsStatus?.current_ipv6 || 'No IPv6 Route'}
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
              <div className="text-[10px] text-slate-500 uppercase">Last Sync Timestamp</div>
              <div className="text-slate-300 font-bold mt-1 truncate">
                {ddnsStatus?.last_sync_time ? new Date(ddnsStatus.last_sync_time).toLocaleTimeString() : 'Never'}
              </div>
            </div>

            <div className="bg-obsidian-950 p-3 rounded-lg border border-obsidian-800 flex items-center justify-between">
              <div>
                <div className="text-[10px] text-slate-500 uppercase">Provider / Status</div>
                <div className="text-emerald-400 font-bold mt-1 capitalize truncate">
                  {ddnsProvider}
                </div>
              </div>
              <button
                type="button"
                onClick={handleSyncDDNS}
                disabled={syncingDDNS || !ddnsEnabled}
                className="px-2.5 py-1.5 bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 rounded-lg text-xs font-bold flex items-center gap-1.5 transition-colors disabled:opacity-50"
                title="Trigger immediate DNS sync"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${syncingDDNS ? 'animate-spin' : ''}`} />
                <span>Sync Now</span>
              </button>
            </div>
          </div>

          {/* Error Banner if Last Sync Failed */}
          {ddnsStatus?.last_error && ddnsStatus.status === 'error' && (
            <div className="p-3 bg-rose-950/30 border border-rose-500/40 rounded-lg flex items-start gap-2 text-rose-300 text-xs font-mono">
              <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
              <div>
                <span className="font-bold">Last Sync Error: </span>
                <span>{ddnsStatus.last_error}</span>
              </div>
            </div>
          )}

          {/* Test / Action Message */}
          {ddnsMessage && (
            <div className={`p-3 rounded-lg border flex items-center gap-2 text-xs font-mono ${
              ddnsMessage.type === 'success'
                ? 'bg-emerald-950/30 border-emerald-500/40 text-emerald-300'
                : 'bg-rose-950/30 border-rose-500/40 text-rose-300'
            }`}>
              {ddnsMessage.type === 'success' ? (
                <Check className="w-4 h-4 text-emerald-400 shrink-0" />
              ) : (
                <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
              )}
              <span>{ddnsMessage.text}</span>
            </div>
          )}

          {/* Form Settings */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs">
            {/* Provider Selector */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">DDNS Provider</label>
              <select
                value={ddnsProvider}
                onChange={(e) => setDdnsProvider(e.target.value as any)}
                className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              >
                <option value="cloudflare">Cloudflare DNS (v4 API)</option>
                <option value="duckdns">DuckDNS (duckdns.org token)</option>
                <option value="noip">No-IP / DynDNS2 (HTTP Basic Auth)</option>
                <option value="webhook">Custom Webhook (HTTP URL Template)</option>
              </select>
              <p className="text-[11px] text-slate-400 mt-1">
                Select your DNS hosting service. Cloudflare and DuckDNS are ideal for Minecraft BDS hosts.
              </p>
            </div>

            {/* Target Domain / Hostname */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">Domain / Hostname</label>
              <input
                type="text"
                placeholder={ddnsProvider === 'duckdns' ? 'e.g. mybds or mybds.duckdns.org' : 'e.g. play.example.com'}
                value={ddnsDomain}
                onChange={(e) => setDdnsDomain(e.target.value)}
                className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                {ddnsProvider === 'duckdns'
                  ? 'Your DuckDNS subdomain name (with or without .duckdns.org).'
                  : 'The fully qualified domain name (FQDN) assigned to your Minecraft server.'}
              </p>
            </div>

            {/* Provider-Specific Credential Inputs */}
            {ddnsProvider === 'cloudflare' && (
              <>
                <div>
                  <label className="block text-slate-200 font-bold mb-1.5 flex items-center justify-between">
                    <span>Cloudflare API Token</span>
                    {ddnsConfigured && (
                      <span className="text-[10px] text-emerald-400 font-normal">Configured</span>
                    )}
                  </label>
                  <div className="relative">
                    <input
                      type={showDdnsToken ? 'text' : 'password'}
                      placeholder={ddnsConfigured ? '****** (Unchanged)' : 'Enter Cloudflare Zone:DNS Edit API Token'}
                      value={ddnsAuthToken}
                      onChange={(e) => setDdnsAuthToken(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => setShowDdnsToken(!showDdnsToken)}
                      className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
                    >
                      {showDdnsToken ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                  <p className="text-[11px] text-slate-400 mt-1">
                    API Token with <strong>Zone.DNS:Edit</strong> permission for your domain.
                  </p>
                </div>

                <div>
                  <label className="block text-slate-200 font-bold mb-1.5">Cloudflare Zone ID</label>
                  <input
                    type="text"
                    placeholder="32-character Cloudflare Zone ID"
                    value={ddnsZoneId}
                    onChange={(e) => setDdnsZoneId(e.target.value)}
                    className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  />
                  <p className="text-[11px] text-slate-400 mt-1">
                    Found on your domain's Overview page in the Cloudflare dashboard.
                  </p>
                </div>
              </>
            )}

            {ddnsProvider === 'duckdns' && (
              <div>
                <label className="block text-slate-200 font-bold mb-1.5 flex items-center justify-between">
                  <span>DuckDNS Token</span>
                  {ddnsConfigured && (
                    <span className="text-[10px] text-emerald-400 font-normal">Configured</span>
                  )}
                </label>
                <div className="relative">
                  <input
                    type={showDdnsToken ? 'text' : 'password'}
                    placeholder={ddnsConfigured ? '****** (Unchanged)' : 'Your DuckDNS Account Token'}
                    value={ddnsAuthToken}
                    onChange={(e) => setDdnsAuthToken(e.target.value)}
                    className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => setShowDdnsToken(!showDdnsToken)}
                    className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
                  >
                    {showDdnsToken ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
                <p className="text-[11px] text-slate-400 mt-1">
                  Found at the top of your duckdns.org account dashboard.
                </p>
              </div>
            )}

            {ddnsProvider === 'noip' && (
              <>
                <div>
                  <label className="block text-slate-200 font-bold mb-1.5">No-IP Username / Email</label>
                  <input
                    type="text"
                    placeholder="Username or email address"
                    value={ddnsUsername}
                    onChange={(e) => setDdnsUsername(e.target.value)}
                    className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  />
                </div>

                <div>
                  <label className="block text-slate-200 font-bold mb-1.5">No-IP Password</label>
                  <div className="relative">
                    <input
                      type={showDdnsPassword ? 'text' : 'password'}
                      placeholder={ddnsConfigured ? '****** (Unchanged)' : 'Account or DDNS Key password'}
                      value={ddnsPassword}
                      onChange={(e) => setDdnsPassword(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => setShowDdnsPassword(!showDdnsPassword)}
                      className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
                    >
                      {showDdnsPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
              </>
            )}

            {ddnsProvider === 'webhook' && (
              <>
                <div>
                  <label className="block text-slate-200 font-bold mb-1.5">Webhook URL Template</label>
                  <input
                    type="text"
                    placeholder="https://api.myhost.com/update?ip={ip}&domain={domain}"
                    value={ddnsWebhookUrl}
                    onChange={(e) => setDdnsWebhookUrl(e.target.value)}
                    className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  />
                  <p className="text-[11px] text-slate-400 mt-1">
                    Placeholders supported: <code>{'{ip}'}</code>, <code>{'{ipv4}'}</code>, <code>{'{ipv6}'}</code>, <code>{'{domain}'}</code>.
                  </p>
                </div>

                <div>
                  <label className="block text-slate-200 font-bold mb-1.5">HTTP Method</label>
                  <select
                    value={ddnsWebhookMethod}
                    onChange={(e) => setDdnsWebhookMethod(e.target.value as any)}
                    className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  >
                    <option value="GET">GET</option>
                    <option value="POST">POST</option>
                  </select>
                </div>
              </>
            )}

            {/* Check Interval */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">Check Interval</label>
              <select
                value={ddnsIntervalMinutes}
                onChange={(e) => setDdnsIntervalMinutes(parseInt(e.target.value, 10))}
                className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              >
                <option value={1}>Every 1 minute (Aggressive)</option>
                <option value={5}>Every 5 minutes (Recommended)</option>
                <option value={10}>Every 10 minutes</option>
                <option value={15}>Every 15 minutes</option>
                <option value={30}>Every 30 minutes</option>
                <option value={60}>Every 1 hour</option>
              </select>
              <p className="text-[11px] text-slate-400 mt-1">
                How frequently BSM polls redundant public IP endpoints for IP changes.
              </p>
            </div>

            {/* IP Stack Options & Auto-Sync Toggle */}
            <div className="space-y-3 pt-1">
              <span className="block text-slate-200 font-bold">DNS Record Types &amp; Propagation</span>
              <div className="space-y-2">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={ddnsEnableIPv4}
                    onChange={(e) => setDdnsEnableIPv4(e.target.checked)}
                    className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-0"
                  />
                  <span className="text-slate-200">Synchronize IPv4 Address (A Record)</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={ddnsEnableIPv6}
                    onChange={(e) => setDdnsEnableIPv6(e.target.checked)}
                    className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-0"
                  />
                  <span className="text-slate-200">Synchronize IPv6 Address (AAAA Record)</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer pt-1 border-t border-obsidian-800">
                  <input
                    type="checkbox"
                    checked={ddnsAutoSyncGameAddress}
                    onChange={(e) => setDdnsAutoSyncGameAddress(e.target.checked)}
                    className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-0"
                  />
                  <span className="text-slate-200">Auto-update Default Game Server Address with DDNS domain</span>
                </label>
              </div>
            </div>
          </div>

          {/* Test & Actions Bar */}
          <div className="flex flex-wrap items-center justify-between gap-3 pt-2">
            <button
              type="button"
              onClick={handleTestDDNS}
              disabled={testingDDNS}
              className="px-4 py-2 bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-200 font-mono text-xs font-bold rounded-lg flex items-center gap-2 transition-colors disabled:opacity-50"
            >
              <Send className={`w-3.5 h-3.5 text-cyan-400 ${testingDDNS ? 'animate-spin' : ''}`} />
              <span>{testingDDNS ? 'Verifying Credentials...' : 'Test Connection / Verify Credentials'}</span>
            </button>
            <div className="text-[11px] font-mono text-slate-400">
              {ddnsProvider === 'cloudflare' && 'Note: Cloudflare records are automatically set to DNS-only (Grey Cloud) for Bedrock UDP compatibility.'}
              {ddnsProvider === 'duckdns' && 'DuckDNS automatically points your domain to your detected public IP.'}
            </div>
          </div>
          </>
        )}
        </div>

        {/* 3. COMMUNITY ADDON MARKETPLACE (CURSEFORGE) */}
        <div id="curseforge" className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
                <Package className="w-4 h-4 text-emerald-400" />
                <span>CurseForge Marketplace &amp; Addon Integration</span>
              </h3>
              <p className="text-xs text-slate-400 font-mono">
                Connect your CurseForge Eternal API key to search, browse, and 1-click install official Minecraft Bedrock addons, resource packs, and worlds.
              </p>
            </div>
            <div>
              {curseForgeConfigured ? (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-mono font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                  <Check className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Configured</span>
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-mono font-bold bg-amber-500/10 text-amber-400 border border-amber-500/30">
                  <AlertCircle className="w-3.5 h-3.5 text-amber-400" />
                  <span>Not Configured</span>
                </span>
              )}
            </div>
          </div>

          {cfMessage && (
            <div
              className={`p-3 rounded-lg border font-mono text-xs flex items-center justify-between ${
                cfMessage.type === 'success'
                  ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-300'
                  : 'bg-rose-950/40 border-rose-500/40 text-rose-300'
              }`}
            >
              <span>{cfMessage.text}</span>
              <button
                type="button"
                onClick={() => setCfMessage(null)}
                className="opacity-70 hover:opacity-100 font-bold ml-2"
              >
                ✕
              </button>
            </div>
          )}

          <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-4">
            <div>
              <label className="block text-slate-200 font-bold mb-1.5 flex items-center justify-between">
                <span className="flex items-center gap-1.5">
                  <Key className="w-3.5 h-3.5 text-amber-400" />
                  <span>CurseForge Eternal API Key</span>
                </span>
                <a
                  href="https://console.curseforge.com/"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[11px] text-emerald-400 hover:underline flex items-center gap-1"
                >
                  <span>console.curseforge.com</span>
                  <ExternalLink className="w-3 h-3" />
                </a>
              </label>

              <div className="relative">
                <input
                  type={showCfKey ? 'text' : 'password'}
                  placeholder={curseForgeConfigured ? '•••••••••••••••••••••••••••••••• (Leave blank to keep existing key)' : 'Enter your CurseForge API key'}
                  value={curseforgeApiKey}
                  onChange={(e) => setCurseforgeApiKey(e.target.value)}
                  className="w-full pr-10 px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
                <button
                  type="button"
                  onClick={() => setShowCfKey(!showCfKey)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
                  title={showCfKey ? 'Hide key' : 'Show key'}
                >
                  {showCfKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>

              <p className="text-[11px] text-slate-400 mt-1.5 leading-relaxed">
                CurseForge uses the Eternal API to search Bedrock Addons and resolve direct CDN downloads. Register a free developer account at{' '}
                <a href="https://console.curseforge.com" target="_blank" rel="noopener noreferrer" className="text-emerald-400 underline">
                  console.curseforge.com
                </a>{' '}
                to obtain your personal API key.
              </p>
            </div>

            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pt-3 border-t border-obsidian-850">
              <span className="text-[11px] text-slate-400">
                {curseForgeConfigured
                  ? 'Key is currently active. You can enter a new key above to overwrite it, or clear it.'
                  : 'Key is not configured. Community addon downloads in Server Hub will remain disabled.'}
              </span>
              <div className="flex items-center gap-2 self-end sm:self-auto">
                {curseForgeConfigured && (
                  <button
                    type="button"
                    onClick={handleClearCurseForgeKey}
                    disabled={savingCfKey}
                    className="px-3.5 py-1.5 rounded-lg bg-rose-950/40 hover:bg-rose-900/50 border border-rose-600/40 text-rose-300 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-40"
                  >
                    <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                    <span>Clear Key</span>
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => handleSaveCurseForgeKey()}
                  disabled={savingCfKey || !curseforgeApiKey.trim()}
                  className="px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-40 shadow-sm"
                >
                  {savingCfKey ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
                  <span>Save CurseForge Key</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* 4. PORT GATE FIREWALL & DYNAMIC LEASE DEFAULTS */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
              <Shield className="w-4 h-4 text-emerald-400" />
              <span>Port Gate Firewall &amp; Lease Defaults</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono">
              Control the baseline firewall lease duration granted to players when unlocking via Knock or temporary keys.
            </p>
          </div>

          <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs">
            <div className="max-w-md space-y-2">
              <label className="block text-slate-200 font-bold">
                Default Dynamic Lease Duration
              </label>
              <div className="flex items-center gap-2">
                <input
                  type="number"
                  min="1"
                  max="365"
                  value={leaseVal}
                  onChange={(e) => setLeaseVal(Math.max(1, parseInt(e.target.value) || 1))}
                  className="w-28 px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
                <select
                  value={leaseUnit}
                  onChange={(e) => setLeaseUnit(e.target.value as DurationUnit)}
                  className="px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                >
                  <option value="minutes">Minutes</option>
                  <option value="hours">Hours</option>
                  <option value="days">Days</option>
                </select>
                <span className="text-[11px] text-slate-400">
                  (= {durationToSeconds(leaseVal, leaseUnit)} seconds)
                </span>
              </div>
              <p className="text-[11px] text-slate-400 pt-1">
                New servers and keys default to this firewall lease window before player packets are blocked again.
              </p>
            </div>
          </div>
        </div>

        {/* 5. BRUTE-FORCE & SECURITY PROTECTION */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
              <ShieldAlert className="w-4 h-4 text-emerald-400" />
              <span>Brute-Force &amp; Security Protection</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono">
              Configure rate limiting, IP lockouts, distributed DDoS circuit breakers, and automated security alerts.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs">
            {/* Rate limit max attempts */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Per-IP Rate Limit Threshold (attempts)
              </label>
              <input
                type="number"
                min="2"
                max="50"
                value={rateLimitMaxAttempts}
                onChange={(e) => setRateLimitMaxAttempts(Math.max(1, parseInt(e.target.value) || 5))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Number of failed login or knock passphrase attempts allowed within 5 minutes before an IP is temporarily locked out (default: 5).
              </p>
            </div>

            {/* Rate limit lockout duration */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Per-IP Lockout Duration (seconds)
              </label>
              <input
                type="number"
                min="30"
                max="86400"
                value={rateLimitLockoutSec}
                onChange={(e) => setRateLimitLockoutSec(Math.max(10, parseInt(e.target.value) || 300))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Duration an offending client IP address remains blocked after exceeding maximum failed attempts (default: 300s / 5m).
              </p>
            </div>

            {/* Circuit breaker threshold */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Distributed Knock Circuit Breaker Threshold (unique IPs)
              </label>
              <input
                type="number"
                min="3"
                max="100"
                value={circuitBreakerThreshold}
                onChange={(e) => setCircuitBreakerThreshold(Math.max(1, parseInt(e.target.value) || 10))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Triggers global Knock portal lockdown if N distinct client IPs submit failed passphrases within a 5-minute rolling window (default: 10).
              </p>
            </div>

            {/* Circuit breaker lockout duration */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Circuit Breaker Global Lockout (minutes)
              </label>
              <input
                type="number"
                min="1"
                max="1440"
                value={circuitBreakerLockoutMin}
                onChange={(e) => setCircuitBreakerLockoutMin(Math.max(1, parseInt(e.target.value) || 15))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Global cool-off period where all public Knock requests are blocked during a coordinated distributed brute-force attack (default: 15m).
              </p>
            </div>

            {/* Discord Security Alerts Toggle */}
            <div className="md:col-span-2 border-t border-obsidian-800/80 pt-4">
              <div className="flex items-center justify-between mb-1.5">
                <span className="font-bold text-slate-200">Discord Security Incident Alerts</span>
                <button
                  type="button"
                  onClick={() => setSecurityAlertDiscord(!securityAlertDiscord)}
                  className="text-emerald-400 focus:outline-none"
                >
                  {securityAlertDiscord ? (
                    <ToggleRight className="w-6 h-6 text-emerald-400" />
                  ) : (
                    <ToggleLeft className="w-6 h-6 text-slate-600" />
                  )}
                </button>
              </div>
              <p className="text-[11px] text-slate-400">
                {securityAlertDiscord
                  ? 'Enabled: Instantly dispatches rich Discord webhook embeds when failed admin logins occur or when Knock passphrases fail / circuit breaker trips.'
                  : 'Disabled: Security incidents are recorded in the audit log only and will not trigger outbound Discord alerts.'}
              </p>
            </div>
          </div>
        </div>

        {/* 5b. CLOUDFLARE TURNSTILE BOT PROTECTION */}
        <div className={`bg-obsidian-900 border border-obsidian-700/80 rounded-xl transition-all ${isTurnstileOpen ? 'p-6 space-y-6' : 'p-5'}`}>
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-emerald-400" />
                <span>Cloudflare Turnstile Bot Protection</span>
              </h3>
              <p className="text-xs text-slate-400 font-mono">
                Defend authentication, public registration, onboarding wizard, and port knocking from automated bots and credential stuffing.
              </p>
            </div>
            <div className="flex items-center gap-2.5 flex-shrink-0">
              <span className={`px-2.5 py-1 rounded-full text-[10px] font-mono font-bold uppercase tracking-wider border ${
                !turnstileEnabled
                  ? 'bg-slate-800 text-slate-400 border-slate-700'
                  : turnstileConfigured || (turnstileSiteKey && turnstileSecretKey)
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/30'
              }`}>
                {!turnstileEnabled ? 'Disabled' : turnstileConfigured || (turnstileSiteKey && turnstileSecretKey) ? 'Active' : 'Unconfigured'}
              </span>

              <label className="inline-flex items-center gap-2 cursor-pointer select-none px-3 py-1.5 rounded-lg bg-obsidian-950 border border-obsidian-800 hover:border-obsidian-700 transition-colors">
                <input
                  type="checkbox"
                  checked={turnstileEnabled}
                  onChange={(e) => {
                    const checked = e.target.checked;
                    setTurnstileEnabled(checked);
                    setTurnstileDetailsOpen(checked);
                  }}
                  className="w-4 h-4 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                />
                <span className={`text-xs font-mono font-bold ${turnstileEnabled ? 'text-emerald-400' : 'text-slate-400'}`}>
                  {turnstileEnabled ? 'Enabled' : 'Enable'}
                </span>
              </label>

              <button
                type="button"
                onClick={() => setTurnstileDetailsOpen(!isTurnstileOpen)}
                className="p-1.5 rounded-lg bg-obsidian-950 border border-obsidian-800 hover:border-obsidian-700 text-slate-400 hover:text-slate-200 transition-colors flex items-center gap-1 text-xs font-mono"
                title={isTurnstileOpen ? 'Collapse card' : 'Configure settings'}
                aria-label={isTurnstileOpen ? 'Collapse Turnstile settings' : 'Configure Turnstile settings'}
              >
                <span className="hidden sm:inline text-[11px]">{isTurnstileOpen ? 'Hide' : 'Configure'}</span>
                {isTurnstileOpen ? <ChevronUp className="w-4 h-4 text-slate-400" /> : <ChevronDown className="w-4 h-4 text-slate-400" />}
              </button>
            </div>
          </div>

          {isTurnstileOpen && (
            <div className="space-y-6">
            {/* Keys Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs">
              {/* Site Key */}
              <div>
                <label className="block text-slate-200 font-bold mb-1.5 flex items-center justify-between">
                  <span>Turnstile Site Key (Public)</span>
                  <a
                    href="https://dash.cloudflare.com/?to=/:account/turnstile"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-emerald-400 hover:underline inline-flex items-center gap-1 text-[11px] font-normal"
                  >
                    <span>Cloudflare Dash</span>
                    <ExternalLink className="w-3 h-3" />
                  </a>
                </label>
                <input
                  type="text"
                  value={turnstileSiteKey}
                  onChange={(e) => setTurnstileSiteKey(e.target.value)}
                  placeholder="0x4AAAAAAA..."
                  className="w-full px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 font-mono text-xs"
                />
                <p className="text-[11px] text-slate-400 mt-1">
                  The public client-side site key generated in your Cloudflare dashboard for Managed challenge mode.
                </p>
              </div>

              {/* Secret Key */}
              <div>
                <label className="block text-slate-200 font-bold mb-1.5 flex items-center justify-between">
                  <span>Turnstile Secret Key (Server Verification)</span>
                  {turnstileConfigured && (
                    <span className="text-[11px] text-emerald-400 bg-emerald-950/60 border border-emerald-500/40 px-2 py-0.5 rounded font-normal">
                      Configured
                    </span>
                  )}
                </label>
                <div className="relative">
                  <input
                    type={showTurnstileSecret ? 'text' : 'password'}
                    value={turnstileSecretKey}
                    onChange={(e) => setTurnstileSecretKey(e.target.value)}
                    placeholder={turnstileConfigured ? '•••••••••••••••• (leave empty to keep current)' : '0x4AAAAAAA...'}
                    className="w-full pl-3 pr-10 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 font-mono text-xs"
                  />
                  <button
                    type="button"
                    onClick={() => setShowTurnstileSecret(!showTurnstileSecret)}
                    className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
                  >
                    {showTurnstileSecret ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
                <p className="text-[11px] text-slate-400 mt-1">
                  Private secret key used strictly by the backend to verify client challenge tokens with Cloudflare.
                </p>
              </div>
            </div>

            {/* Granular Feature Toggles */}
            <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-4">
              <span className="text-slate-200 font-bold block border-b border-obsidian-800 pb-2">
                Protected Flows &amp; Endpoints
              </span>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {/* Login */}
                <label className="flex items-start gap-3 p-2.5 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                  <input
                    type="checkbox"
                    checked={turnstileLoginEnabled}
                    onChange={(e) => setTurnstileLoginEnabled(e.target.checked)}
                    className="mt-0.5 rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500/30"
                  />
                  <div>
                    <span className="font-bold text-slate-200 block">User &amp; Admin Login</span>
                    <span className="text-[11px] text-slate-400 block mt-0.5">Enforces verification on <code className="text-emerald-400 font-mono">/api/auth/login</code></span>
                  </div>
                </label>

                {/* Registration */}
                <label className="flex items-start gap-3 p-2.5 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                  <input
                    type="checkbox"
                    checked={turnstileRegisterEnabled}
                    onChange={(e) => setTurnstileRegisterEnabled(e.target.checked)}
                    className="mt-0.5 rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500/30"
                  />
                  <div>
                    <span className="font-bold text-slate-200 block">Public Registration</span>
                    <span className="text-[11px] text-slate-400 block mt-0.5">Enforces verification on <code className="text-emerald-400 font-mono">/api/auth/register</code></span>
                  </div>
                </label>

                {/* Setup */}
                <label className="flex items-start gap-3 p-2.5 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                  <input
                    type="checkbox"
                    checked={turnstileSetupEnabled}
                    onChange={(e) => setTurnstileSetupEnabled(e.target.checked)}
                    className="mt-0.5 rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500/30"
                  />
                  <div>
                    <span className="font-bold text-slate-200 block">Initial Setup Wizard</span>
                    <span className="text-[11px] text-slate-400 block mt-0.5">Enforces verification on first-time <code className="text-emerald-400 font-mono">/api/setup</code></span>
                  </div>
                </label>

                {/* Knock Portal */}
                <label className="flex items-start gap-3 p-2.5 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                  <input
                    type="checkbox"
                    checked={turnstileKnockEnabled}
                    onChange={(e) => setTurnstileKnockEnabled(e.target.checked)}
                    className="mt-0.5 rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500/30"
                  />
                  <div>
                    <span className="font-bold text-slate-200 block">Port Gate Knock Portal</span>
                    <span className="text-[11px] text-slate-400 block mt-0.5">Enforces verification on public knock requests before opening firewall</span>
                  </div>
                </label>
              </div>

              {/* Fail-Open Toggle */}
              <div className="border-t border-obsidian-800/80 pt-3 flex items-start justify-between gap-4">
                <div>
                  <span className="font-bold text-slate-200 block">Fail-Open on Cloudflare Outages</span>
                  <span className="text-[11px] text-slate-400 block mt-0.5">
                    {turnstileFailOpen
                      ? 'Enabled: If Cloudflare verification servers are completely unreachable or timing out, requests will proceed with an audit warning to prevent admin lockout.'
                      : 'Disabled (Strict): Protected requests will be blocked if Cloudflare siteverify is unreachable.'}
                  </span>
                </div>
                <button
                  type="button"
                  onClick={() => setTurnstileFailOpen(!turnstileFailOpen)}
                  className="text-emerald-400 focus:outline-none flex-shrink-0"
                >
                  {turnstileFailOpen ? (
                    <ToggleRight className="w-6 h-6 text-emerald-400" />
                  ) : (
                    <ToggleLeft className="w-6 h-6 text-slate-600" />
                  )}
                </button>
              </div>
            </div>

            {/* Interactive In-Page Turnstile Verification Tester */}
            <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-slate-200 font-bold block">Live Configuration Verification Tester</span>
                  <span className="text-[11px] text-slate-400 block">
                    Solve the challenge widget below to test your Site Key and Secret Key pair against Cloudflare before saving.
                  </span>
                </div>
                <button
                  type="button"
                  disabled={testingTurnstile || !turnstileTestToken}
                  onClick={handleTestTurnstile}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold text-xs transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  {testingTurnstile ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <ShieldCheck className="w-3.5 h-3.5" />}
                  <span>Verify Key Pair</span>
                </button>
              </div>

              {turnstileTestMessage && (
                <div
                  className={`p-3 rounded-lg flex items-start gap-2.5 text-xs ${
                    turnstileTestMessage.type === 'success'
                      ? 'bg-emerald-950/40 border border-emerald-500/50 text-emerald-300'
                      : 'bg-rose-950/40 border border-rose-500/50 text-rose-300'
                  }`}
                >
                  {turnstileTestMessage.type === 'success' ? (
                    <Check className="w-4 h-4 text-emerald-400 flex-shrink-0 mt-0.5" />
                  ) : (
                    <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0 mt-0.5" />
                  )}
                  <span>{turnstileTestMessage.text}</span>
                </div>
              )}

              {turnstileSiteKey.trim() ? (
                <div className="pt-2 flex flex-col items-center">
                  <TurnstileWidget
                    ref={turnstileTestRef}
                    siteKey={turnstileSiteKey.trim()}
                    action="settings_test"
                    onVerify={(token) => {
                      setTurnstileTestToken(token);
                      setTurnstileTestMessage(null);
                    }}
                    onExpire={() => setTurnstileTestToken('')}
                    onError={(err) => {
                      setTurnstileTestToken('');
                      setTurnstileTestMessage({ text: err || 'Challenge loading error', type: 'error' });
                    }}
                  />
                  {turnstileTestToken && (
                    <span className="text-[11px] text-emerald-400 mt-1 flex items-center gap-1">
                      <Check className="w-3 h-3" />
                      <span>Challenge solved! Ready to verify with Cloudflare.</span>
                    </span>
                  )}
                </div>
              ) : (
                <div className="py-4 text-center text-slate-500 text-xs italic">
                  Enter a Turnstile Site Key above to load and test the verification challenge widget.
                </div>
              )}
            </div>
          </div>
        )}
        </div>

        {/* 6. RESOURCE MONITORING & PERFORMANCE ALERTS */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
              <Activity className="w-4 h-4 text-emerald-400" />
              <span>Resource Monitoring &amp; Alert Thresholds</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono">
              Configure system utilization guardrails to detect instance spikes, memory exhaustion, or runaway servers.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs">
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Warning Threshold (%)
              </label>
              <div className="flex items-center gap-2">
                <input
                  type="number"
                  min="50"
                  max="99"
                  value={alertThreshold}
                  onChange={(e) => setAlertThreshold(Math.min(99, Math.max(50, parseInt(e.target.value) || 85)))}
                  className="w-28 px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
                <span className="text-slate-400 font-bold">%</span>
              </div>
              <p className="text-[11px] text-slate-400 mt-1">
                Triggers in-game chat warning and webhook alert when instance CPU or RAM exceeds this percentage (default: 85%).
              </p>
            </div>

            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Alert Notification Cooldown (seconds)
              </label>
              <input
                type="number"
                min="10"
                max="600"
                value={alertIntervalSec}
                onChange={(e) => setAlertIntervalSec(Math.max(10, parseInt(e.target.value) || 60))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Minimum cooldown period between repeated resource warning broadcasts per server (default: 60s).
              </p>
            </div>
          </div>
        </div>

        {/* 7. DISCORD WEBHOOK & REAL-TIME NOTIFICATIONS */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
              <Bell className="w-4 h-4 text-emerald-400" />
              <span>Discord Webhook &amp; Outbound Notifications</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono">
              Push real-time embeds to your Discord channel for server start/stops, crash reports, player join/leaves, and resource warnings.
            </p>
          </div>

          {webhookTestMessage && (
            <div
              className={`p-3 rounded-lg border font-mono text-xs flex items-center justify-between ${
                webhookTestMessage.type === 'success'
                  ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-300'
                  : 'bg-rose-950/40 border-rose-500/40 text-rose-300'
              }`}
            >
              <span>{webhookTestMessage.text}</span>
              <button
                type="button"
                onClick={() => setWebhookTestMessage(null)}
                className="opacity-70 hover:opacity-100 font-bold ml-2"
              >
                ✕
              </button>
            </div>
          )}

          <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-3">
            <div className="flex items-center justify-between mb-1">
              <span className="font-bold text-slate-200">Discord Webhook URL</span>
              {discordConfigured && (
                <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30">
                  Configured
                </span>
              )}
            </div>
            <input
              type="password"
              placeholder={discordConfigured ? '•••••••••••••••• (Leave blank to keep existing URL)' : 'https://discord.com/api/webhooks/...'}
              value={discordWebhookUrl}
              onChange={(e) => setDiscordWebhookUrl(e.target.value)}
              className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
            />
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pt-2">
              <p className="text-[11px] text-slate-400">
                Created in Discord via Channel Settings → Integrations → Webhooks.
              </p>
              <button
                type="button"
                onClick={handleTestDiscordWebhook}
                disabled={testingWebhook || (!discordWebhookUrl && !discordConfigured)}
                className="px-3.5 py-1.5 rounded-lg bg-obsidian-850 hover:bg-obsidian-750 border border-obsidian-700 text-slate-200 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-40"
              >
                {testingWebhook ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Send className="w-3.5 h-3.5 text-indigo-400" />}
                <span>Send Test Notification</span>
              </button>
            </div>
          </div>
        </div>

        {/* 8. SMTP & OUTBOUND EMAIL DELIVERY */}
        <div className={`bg-obsidian-900 border border-obsidian-700/80 rounded-xl transition-all ${isSmtpOpen ? 'p-6 space-y-6' : 'p-5'}`}>
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
                <Mail className="w-4 h-4 text-emerald-400" />
                <span>SMTP & Outbound Email Delivery</span>
              </h3>
              <p className="text-xs text-slate-400 font-mono">
                Configure mail transport credentials for password resets, email verification, and administrative alerts.
              </p>
            </div>
            <div className="flex items-center gap-2.5 flex-shrink-0">
              <span className={`px-2.5 py-1 rounded-full text-[10px] font-mono font-bold uppercase tracking-wider border ${
                !smtpEnabled
                  ? 'bg-slate-800 text-slate-400 border-slate-700'
                  : smtpHost
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/30'
              }`}>
                {!smtpEnabled ? 'Disabled' : smtpHost ? 'Active' : 'Incomplete'}
              </span>

              <label className="inline-flex items-center gap-2 cursor-pointer select-none px-3 py-1.5 rounded-lg bg-obsidian-950 border border-obsidian-800 hover:border-obsidian-700 transition-colors">
                <input
                  type="checkbox"
                  checked={smtpEnabled}
                  onChange={(e) => {
                    const checked = e.target.checked;
                    setSmtpEnabled(checked);
                    setSmtpDetailsOpen(checked);
                  }}
                  className="w-4 h-4 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                />
                <span className={`text-xs font-mono font-bold ${smtpEnabled ? 'text-emerald-400' : 'text-slate-400'}`}>
                  {smtpEnabled ? 'Enabled' : 'Enable'}
                </span>
              </label>

              <button
                type="button"
                onClick={() => setSmtpDetailsOpen(!isSmtpOpen)}
                className="p-1.5 rounded-lg bg-obsidian-950 border border-obsidian-800 hover:border-obsidian-700 text-slate-400 hover:text-slate-200 transition-colors flex items-center gap-1 text-xs font-mono"
                title={isSmtpOpen ? 'Collapse card' : 'Configure settings'}
                aria-label={isSmtpOpen ? 'Collapse SMTP settings' : 'Configure SMTP settings'}
              >
                <span className="hidden sm:inline text-[11px]">{isSmtpOpen ? 'Hide' : 'Configure'}</span>
                {isSmtpOpen ? <ChevronUp className="w-4 h-4 text-slate-400" /> : <ChevronDown className="w-4 h-4 text-slate-400" />}
              </button>
            </div>
          </div>

          {isSmtpOpen && (
            <div className="space-y-6">
              {/* Test Status Banner */}
              {smtpTestMessage && (
                <div
                  className={`p-3.5 rounded-xl border font-mono text-xs flex items-center justify-between gap-3 ${
                    smtpTestMessage.type === 'success'
                      ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                      : 'bg-red-500/10 border-red-500/30 text-red-400'
                  }`}
                >
                  <div className="flex items-center gap-2">
                    {smtpTestMessage.type === 'success' ? (
                      <Check className="w-4 h-4 flex-shrink-0" />
                    ) : (
                      <AlertCircle className="w-4 h-4 flex-shrink-0" />
                    )}
                    <span>{smtpTestMessage.text}</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setSmtpTestMessage(null)}
                    className="opacity-70 hover:opacity-100 font-bold ml-2"
                  >
                    ✕
                  </button>
                </div>
              )}

              {/* 1. Connection & Encryption */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-4">
                <div className="text-slate-200 font-bold flex items-center gap-2">
                  <Globe className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Mail Transport & Server Connection</span>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div>
                    <label className="block text-slate-300 font-bold mb-1.5">
                      SMTP Host / Relay
                    </label>
                    <input
                      type="text"
                      placeholder="e.g. smtp.mailgun.org or smtp.gmail.com"
                      value={smtpHost}
                      onChange={(e) => setSmtpHost(e.target.value)}
                      className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                    <p className="text-[11px] text-slate-500 mt-1">Hostname or IP of outbound mail server.</p>
                  </div>

                  <div>
                    <label className="block text-slate-300 font-bold mb-1.5">
                      Port
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="65535"
                      value={smtpPort}
                      onChange={(e) => setSmtpPort(parseInt(e.target.value, 10) || 587)}
                      className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                    <p className="text-[11px] text-slate-500 mt-1">Common: 587 (STARTTLS), 465 (SSL/TLS), 25 (Plain).</p>
                  </div>

                  <div>
                    <label className="block text-slate-300 font-bold mb-1.5">
                      Transport Encryption
                    </label>
                    <select
                      value={smtpEncryption}
                      onChange={(e) => {
                        const val = e.target.value;
                        setSmtpEncryption(val);
                        if (val === 'tls' && smtpPort === 587) setSmtpPort(465);
                        if (val === 'starttls' && smtpPort === 465) setSmtpPort(587);
                        if (val === 'none' && (smtpPort === 587 || smtpPort === 465)) setSmtpPort(25);
                      }}
                      className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    >
                      <option value="starttls">STARTTLS (Standard / Port 587)</option>
                      <option value="tls">Direct SSL / TLS (SMTPS / Port 465)</option>
                      <option value="none">None (Plaintext / Port 25)</option>
                    </select>
                    <p className="text-[11px] text-slate-500 mt-1">STARTTLS recommended for modern relays.</p>
                  </div>
                </div>

                <div className="pt-2 border-t border-obsidian-850">
                  <label className="inline-flex items-center gap-2 cursor-pointer select-none">
                    <input
                      type="checkbox"
                      checked={smtpSkipTlsVerify}
                      onChange={(e) => setSmtpSkipTlsVerify(e.target.checked)}
                      className="w-4 h-4 rounded border-obsidian-700 bg-obsidian-900 text-amber-500 focus:ring-amber-500/20 focus:ring-offset-0 accent-amber-500 cursor-pointer"
                    />
                    <span className="text-xs text-slate-300">
                      Skip TLS Certificate Verification (Insecure / Self-Signed)
                    </span>
                  </label>
                  {smtpSkipTlsVerify && (
                    <p className="text-[11px] text-amber-400 mt-1 flex items-center gap-1">
                      <AlertCircle className="w-3.5 h-3.5" />
                      Certificate validation disabled. Use only for internal or self-hosted test relays.
                    </p>
                  )}
                </div>
              </div>

              {/* 2. Authentication & Credentials */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-4">
                <div className="text-slate-200 font-bold flex items-center gap-2">
                  <Key className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Authentication & Sender Identity</span>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-slate-300 font-bold mb-1.5">
                      Authentication Method
                    </label>
                    <select
                      value={smtpAuthType}
                      onChange={(e) => setSmtpAuthType(e.target.value)}
                      className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    >
                      <option value="plain">Standard Credentials (PLAIN / LOGIN)</option>
                      <option value="none">Anonymous (No Authentication / Local Relay)</option>
                    </select>
                    <p className="text-[11px] text-slate-500 mt-1">Select Anonymous for intranet Postfix / Exim relays.</p>
                  </div>

                  <div>
                    <label className="block text-slate-300 font-bold mb-1.5">
                      Public Application Base URL (app_url)
                    </label>
                    <input
                      type="text"
                      placeholder="https://bsm.yourdomain.com (auto-inferred if blank)"
                      value={appUrl}
                      onChange={(e) => setAppUrl(e.target.value)}
                      className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                    <p className="text-[11px] text-slate-500 mt-1">Used to craft password reset and verification links.</p>
                  </div>
                </div>

                {smtpAuthType !== 'none' && (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-obsidian-850">
                    <div>
                      <label className="block text-slate-300 font-bold mb-1.5">
                        SMTP Username / API Key
                      </label>
                      <input
                        type="text"
                        placeholder="e.g. postmaster@yourdomain.com or apikey"
                        value={smtpUsername}
                        onChange={(e) => setSmtpUsername(e.target.value)}
                        className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                      />
                    </div>

                    <div>
                      <label className="block text-slate-300 font-bold mb-1.5 flex items-center justify-between">
                        <span>SMTP Password / Secret</span>
                        {smtpPasswordConfigured && (
                          <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30">
                            Configured
                          </span>
                        )}
                      </label>
                      <div className="relative">
                        <input
                          type={showSmtpPassword ? 'text' : 'password'}
                          placeholder={smtpPasswordConfigured ? '•••••••••••••••• (Leave blank to keep existing password)' : 'Enter SMTP password or API token'}
                          value={smtpPassword}
                          onChange={(e) => setSmtpPassword(e.target.value)}
                          className="w-full px-3 py-1.5 pr-10 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 font-mono text-xs"
                        />
                        <button
                          type="button"
                          onClick={() => setShowSmtpPassword(!showSmtpPassword)}
                          className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300 p-1"
                        >
                          {showSmtpPassword ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                        </button>
                      </div>
                    </div>
                  </div>
                )}

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-obsidian-850">
                  <div>
                    <label className="block text-slate-300 font-bold mb-1.5">
                      From Email Address
                    </label>
                    <input
                      type="email"
                      placeholder="e.g. noreply@yourdomain.com"
                      value={smtpFromEmail}
                      onChange={(e) => setSmtpFromEmail(e.target.value)}
                      className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                    <p className="text-[11px] text-slate-500 mt-1">Sender address for outbound messages.</p>
                  </div>

                  <div>
                    <label className="block text-slate-300 font-bold mb-1.5">
                      From Display Name
                    </label>
                    <input
                      type="text"
                      placeholder="Bedrock Server Manager"
                      value={smtpFromName}
                      onChange={(e) => setSmtpFromName(e.target.value)}
                      className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                    <p className="text-[11px] text-slate-500 mt-1">Display name shown in user email clients.</p>
                  </div>
                </div>
              </div>

              {/* 3. Account Verification & Welcome Workflow Policies */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-4">
                <div className="text-slate-200 font-bold flex items-center gap-2">
                  <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Account Workflows & User Lifecycle</span>
                </div>

                <div className="space-y-3">
                  <label className="flex items-start gap-3 p-3 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                    <input
                      type="checkbox"
                      checked={smtpRequireEmailVerification}
                      onChange={(e) => setSmtpRequireEmailVerification(e.target.checked)}
                      className="w-4 h-4 mt-0.5 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                    />
                    <div className="flex-1">
                      <span className="block font-bold text-slate-200">Require Email Verification on Registration</span>
                      <span className="block text-[11px] text-slate-400 mt-0.5">
                        New standard users must click a secure verification link sent to their inbox before logging in. Administrator accounts bypass this restriction.
                      </span>
                    </div>
                  </label>

                  <label className="flex items-start gap-3 p-3 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                    <input
                      type="checkbox"
                      checked={smtpSendWelcomeEmail}
                      onChange={(e) => setSmtpSendWelcomeEmail(e.target.checked)}
                      className="w-4 h-4 mt-0.5 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                    />
                    <div className="flex-1">
                      <span className="block font-bold text-slate-200">Send Welcome Onboarding Email</span>
                      <span className="block text-[11px] text-slate-400 mt-0.5">
                        Sends a welcome email with quick-start dashboard tips when a new user joins or verifies their account.
                      </span>
                    </div>
                  </label>
                </div>
              </div>

              {/* 4. Incident & Event Alert Routing */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-4">
                <div className="text-slate-200 font-bold flex items-center gap-2">
                  <Bell className="w-3.5 h-3.5 text-emerald-400" />
                  <span>System Incident Email Routing</span>
                </div>

                <div>
                  <label className="block text-slate-300 font-bold mb-1.5">
                    Alert Recipient Email Addresses
                  </label>
                  <input
                    type="text"
                    placeholder="e.g. alerts@yourdomain.com, oncall@example.com (comma-separated; defaults to all admins)"
                    value={smtpAlertRecipients}
                    onChange={(e) => setSmtpAlertRecipients(e.target.value)}
                    className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  />
                  <p className="text-[11px] text-slate-500 mt-1">
                    Recipients for operational alerts. Leave empty to automatically notify all registered Admin users.
                  </p>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-3 pt-2 border-t border-obsidian-850">
                  <label className="flex items-start gap-2.5 p-3 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                    <input
                      type="checkbox"
                      checked={smtpAlertCrashes}
                      onChange={(e) => setSmtpAlertCrashes(e.target.checked)}
                      className="w-4 h-4 mt-0.5 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                    />
                    <div>
                      <span className="block font-bold text-slate-200">Server Crashes</span>
                      <span className="block text-[10px] text-slate-400 mt-0.5">Instance crashes or non-zero exits</span>
                    </div>
                  </label>

                  <label className="flex items-start gap-2.5 p-3 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                    <input
                      type="checkbox"
                      checked={smtpAlertSecurity}
                      onChange={(e) => setSmtpAlertSecurity(e.target.checked)}
                      className="w-4 h-4 mt-0.5 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                    />
                    <div>
                      <span className="block font-bold text-slate-200">Security & Lockouts</span>
                      <span className="block text-[10px] text-slate-400 mt-0.5">Rate limiter & circuit breaker trips</span>
                    </div>
                  </label>

                  <label className="flex items-start gap-2.5 p-3 rounded-lg bg-obsidian-900/60 border border-obsidian-800 cursor-pointer hover:border-obsidian-700 transition-colors">
                    <input
                      type="checkbox"
                      checked={smtpAlertResources}
                      onChange={(e) => setSmtpAlertResources(e.target.checked)}
                      className="w-4 h-4 mt-0.5 rounded border-obsidian-700 bg-obsidian-900 text-emerald-500 focus:ring-emerald-500/20 focus:ring-offset-0 accent-emerald-500 cursor-pointer"
                    />
                    <div>
                      <span className="block font-bold text-slate-200">Resource Warnings</span>
                      <span className="block text-[10px] text-slate-400 mt-0.5">Host CPU or RAM exceeds threshold</span>
                    </div>
                  </label>
                </div>
              </div>

              {/* 5. Synchronous Test Email Dispatcher */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs space-y-3">
                <div className="flex items-center justify-between">
                  <span className="font-bold text-slate-200 flex items-center gap-1.5">
                    <Send className="w-3.5 h-3.5 text-emerald-400" />
                    <span>Send Test Email</span>
                  </span>
                  <span className="text-[11px] text-slate-500">
                    Executes synchronously with live SMTP server response
                  </span>
                </div>

                <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
                  <input
                    type="email"
                    placeholder="recipient@example.com (defaults to logged-in user or From email)"
                    value={smtpTestRecipient}
                    onChange={(e) => setSmtpTestRecipient(e.target.value)}
                    className="flex-1 px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  />
                  <button
                    type="button"
                    onClick={handleTestSMTP}
                    disabled={testingSMTP || (!smtpHost && !smtpEnabled)}
                    className="px-4 py-1.5 rounded-lg bg-obsidian-850 hover:bg-obsidian-750 border border-obsidian-700 text-slate-200 font-bold text-xs flex items-center justify-center gap-1.5 transition-colors disabled:opacity-40 flex-shrink-0"
                  >
                    {testingSMTP ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Send className="w-3.5 h-3.5 text-emerald-400" />}
                    <span>{testingSMTP ? 'Dispatching...' : 'Send Test Email'}</span>
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* 9. AUTOMATED BACKUP RETENTION POLICIES */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
              <Database className="w-4 h-4 text-emerald-400" />
              <span>Automated Backup Retention Defaults</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono">
              Define system-wide pruning thresholds for unpinned backups to safeguard host storage capacity.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 font-mono text-xs">
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Max Backups per Server
              </label>
              <input
                type="number"
                min="1"
                max="100"
                value={backupRetentionMaxCount}
                onChange={(e) => setBackupRetentionMaxCount(Math.max(1, parseInt(e.target.value) || 10))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Keep at most N newest unpinned backups per server (default: 10).
              </p>
            </div>

            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Retention Window (days)
              </label>
              <input
                type="number"
                min="1"
                max="365"
                value={backupRetentionMaxDays}
                onChange={(e) => setBackupRetentionMaxDays(Math.max(1, parseInt(e.target.value) || 14))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Prune unpinned backups older than N days (default: 14 days).
              </p>
            </div>

            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Max Disk Usage Cap (GB)
              </label>
              <input
                type="number"
                min="1"
                max="500"
                step="0.5"
                value={backupRetentionMaxGB}
                onChange={(e) => setBackupRetentionMaxGB(Math.max(1, parseFloat(e.target.value) || 5))}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Max cumulative unpinned backup storage per server before pruning oldest (default: 5 GB).
              </p>
            </div>
          </div>
        </div>

        {/* 9. BDS ENGINE & WINDOWS HOST SETTINGS CARD */}
        {bdsStatus && (
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
            <div>
              <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
                <Box className="w-4 h-4 text-emerald-400" />
                <span>Mojang Bedrock Dedicated Server (BDS) &amp; Host Configuration</span>
              </h3>
              <p className="text-xs text-slate-400 font-mono">
                Manage official Mojang BDS engine settings, version caches, EULA compliance, and host integrations.
              </p>
            </div>

            {bdsMessage && (
              <div
                className={`p-3 rounded-lg border font-mono text-xs flex items-center justify-between ${
                  bdsMessage.type === 'success'
                    ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-300'
                    : 'bg-rose-950/40 border-rose-500/40 text-rose-300'
                }`}
              >
                <span>{bdsMessage.text}</span>
                <button
                  type="button"
                  onClick={() => setBdsMessage(null)}
                  className="opacity-70 hover:opacity-100 font-bold ml-2"
                >
                  ✕
                </button>
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 font-mono text-xs">
              {/* BDS EULA Compliance */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="font-bold text-slate-200 flex items-center gap-1.5">
                    <ShieldCheck className="w-4 h-4 text-emerald-400" />
                    <span>Minecraft BDS EULA</span>
                  </span>
                  {bdsStatus.eula_accepted ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                      Accepted
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/30">
                      Not Accepted
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Official BDS binaries require agreement to the{' '}
                  <a
                    href={bdsStatus.eula_url}
                    target="_blank"
                    rel="noreferrer"
                    className="underline text-emerald-400 hover:text-emerald-300 inline-flex items-center gap-0.5"
                  >
                    Minecraft EULA <ExternalLink className="w-3 h-3" />
                  </a>{' '}
                  and{' '}
                  <a
                    href={bdsStatus.privacy_url}
                    target="_blank"
                    rel="noreferrer"
                    className="underline text-emerald-400 hover:text-emerald-300 inline-flex items-center gap-0.5"
                  >
                    Privacy Statement <ExternalLink className="w-3 h-3" />
                  </a>.
                </p>
                {bdsStatus.eula_accepted ? (
                  <div className="text-[10px] text-slate-500">
                    Accepted {bdsStatus.eula_accepted_at ? `on ${new Date(bdsStatus.eula_accepted_at).toLocaleString()}` : ''}{' '}
                    {bdsStatus.eula_accepted_by ? `by ${bdsStatus.eula_accepted_by}` : ''}
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={handleAcceptEula}
                    disabled={bdsActionLoading === 'eula'}
                    className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-50"
                  >
                    {bdsActionLoading === 'eula' && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                    <span>Agree &amp; Accept EULA</span>
                  </button>
                )}
              </div>

              {/* Visual C++ Runtime Status */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="font-bold text-slate-200 flex items-center gap-1.5">
                    <Cpu className="w-4 h-4 text-emerald-400" />
                    <span>Visual C++ 2015–2022 Runtime</span>
                  </span>
                  {bdsStatus.vc_installed ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                      Installed
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/30">
                      Missing
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed">
                  {bdsStatus.vc_installed
                    ? 'MSVC v14 runtime library detected in registry / system folder. Native BDS can launch normally.'
                    : 'bedrock_server.exe requires the x64 Microsoft Visual C++ Redistributable to execute on Windows.'}
                </p>
                {!bdsStatus.vc_installed && (
                  <div>
                    <a
                      href="https://aka.ms/vs/17/release/vc_redist.x64.exe"
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-slate-950 font-bold text-xs transition-colors"
                    >
                      <Download className="w-3.5 h-3.5" />
                      <span>Download vc_redist.x64.exe</span>
                    </a>
                  </div>
                )}
              </div>

              {/* BDS Cache & Versions */}
              <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="font-bold text-slate-200 flex items-center gap-1.5">
                    <HardDrive className="w-4 h-4 text-emerald-400" />
                    <span>BDS Binary Cache</span>
                  </span>
                  <span className="text-[10px] text-slate-400">
                    {bdsStatus.cache_count} cached ({(bdsStatus.cache_bytes / (1024 * 1024)).toFixed(1)} MB)
                  </span>
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Cached versions:{' '}
                  {bdsStatus.cached_versions && bdsStatus.cached_versions.length > 0 ? (
                    <span className="text-slate-300">{bdsStatus.cached_versions.join(', ')}</span>
                  ) : (
                    <span className="text-slate-500 italic">None yet downloaded</span>
                  )}
                </p>
                <div className="flex flex-wrap items-center gap-2 pt-1">
                  <button
                    type="button"
                    onClick={handlePurgeCache}
                    disabled={bdsActionLoading === 'purge' || bdsStatus.cache_count === 0}
                    className="px-3 py-1.5 rounded-lg bg-obsidian-850 hover:bg-obsidian-750 border border-obsidian-700 text-slate-300 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-40"
                  >
                    {bdsActionLoading === 'purge' && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                    <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                    <span>Purge Unused Versions</span>
                  </button>
                  <button
                    type="button"
                    onClick={handleCheckVersions}
                    disabled={bdsActionLoading === 'versions'}
                    className="px-3 py-1.5 rounded-lg bg-obsidian-850 hover:bg-obsidian-750 border border-obsidian-700 text-slate-300 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-40"
                  >
                    {bdsActionLoading === 'versions' ? (
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    ) : (
                      <RefreshCw className="w-3.5 h-3.5 text-emerald-400" />
                    )}
                    <span>Check BDS Releases</span>
                  </button>
                </div>
                {bdsVersions && (
                  <div className="text-[10px] text-slate-400 pt-1 border-t border-obsidian-800/60 flex flex-col gap-0.5">
                    <div>Latest Stable: <span className="text-emerald-400 font-bold">{bdsVersions.latest?.version || 'N/A'}</span></div>
                    <div>Latest Preview: <span className="text-cyan-400 font-bold">{bdsVersions.preview?.version || 'N/A'}</span></div>
                  </div>
                )}
              </div>

              {/* Windows Host Integrations */}
              {bdsStatus.is_windows && (
                <div className="p-4 rounded-xl bg-obsidian-950 border border-obsidian-800 space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-slate-200 flex items-center gap-1.5">
                      <Shield className="w-4 h-4 text-emerald-400" />
                      <span>Windows Host Optimizations</span>
                    </span>
                    <span className="text-[10px] text-emerald-400 font-bold bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30">
                      Host: Windows
                    </span>
                  </div>
                  <div className="space-y-3 pt-1">
                    <div>
                      <div className="text-[11px] font-bold text-slate-300 mb-1">
                        Local Client Loopback Exemption
                      </div>
                      <p className="text-[10px] text-slate-400 mb-1.5">
                        Enables the local Minecraft UWP app on this PC to connect to local servers via 127.0.0.1.
                      </p>
                      <button
                        type="button"
                        onClick={handleSetLoopback}
                        disabled={bdsActionLoading === 'loopback'}
                        className="px-3 py-1.5 rounded-lg bg-obsidian-850 hover:bg-obsidian-750 border border-obsidian-700 text-slate-300 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-40"
                      >
                        {bdsActionLoading === 'loopback' && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                        <span>Enable UWP Loopback</span>
                      </button>
                    </div>

                    <div className="border-t border-obsidian-800/80 pt-2.5">
                      <div className="text-[11px] font-bold text-slate-300 mb-1">
                        Windows Defender Exclusion
                      </div>
                      <p className="text-[10px] text-slate-400 mb-1.5">
                        Adds data directory exclusion to prevent antivirus real-time scan lockups during world saves.
                      </p>
                      <button
                        type="button"
                        onClick={handleAddDefender}
                        disabled={bdsActionLoading === 'defender'}
                        className="px-3 py-1.5 rounded-lg bg-obsidian-850 hover:bg-obsidian-750 border border-obsidian-700 text-slate-300 font-bold text-xs flex items-center gap-1.5 transition-colors disabled:opacity-40"
                      >
                        {bdsActionLoading === 'defender' && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                        <span>Add Defender Exclusion</span>
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* BOTTOM SAVE BUTTON */}
        <div className="flex justify-end pt-4">
          <button
            type="submit"
            disabled={savingSettings}
            className="px-6 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-2 shadow-[0_0_15px_rgba(16,185,129,0.25)] transition-all disabled:opacity-50"
          >
            {savingSettings ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
            <span>Save All Settings</span>
          </button>
        </div>
      </form>
    </div>
  );
};
