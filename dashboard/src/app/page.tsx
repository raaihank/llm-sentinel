'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { AlertTriangle, Shield, Activity, Eye, Moon, Sun, Zap, Lock, Globe, Database, ExternalLink, Clock, Server } from 'lucide-react';

interface Detection {
  entityType: string;
  masked: string;
  count: number;
}

interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
  data?: Record<string, unknown>;
}

interface DetectionEvent {
  timestamp: string;
  entityType: string;
  maskedAs: string;
  occurrences: number;
  endpoint: string;
  id?: string;
  provider?: 'OpenAI' | 'Ollama' | 'Cursor' | 'GitHub Copilot' | 'Claude' | 'Unknown';
}

interface DetailedRequestEvent {
  id: string;
  timestamp: string;
  endpoint: string;
  method: string;
  processingTimeMs: number;
  detections: Detection[];
  contentLength: number;
  status: 'processing' | 'completed' | 'error';
  requestHeaders?: Record<string, string>;
  requestBody?: string;
  maskedRequestBody?: string;
  responseHeaders?: Record<string, string>;
  responseBody?: string;
  responseStatus?: number;
  logs?: LogEntry[];
  apiResponseTimeMs?: number;
}

interface RecentMetric {
  id: string;
  timestamp: string;
  endpoint: string;
  detections: Detection[];
}

interface DashboardStats {
  totalRequests: number;
  totalDetections: number;
  activeDetectors: number;
  uptime: string;
  recentEvents: DetectionEvent[];
}

interface DetectionConfig {
  enabled: boolean;
  enabledRules: string[];
}

interface LoggingConfig {
  logLevel: string;
  [key: string]: unknown;
}

interface SecurityConfig {
  redactApiKeys: boolean;
  redactCustomHeaders: string[];
  [key: string]: unknown;
}

interface NotificationConfig {
  enabled: boolean;
  [key: string]: unknown;
}

interface ServerConfig {
  port: number;
  [key: string]: unknown;
}

interface TargetConfig {
  target: string;
  [key: string]: unknown;
}

interface ConfigData {
  detection: DetectionConfig;
  logging: LoggingConfig;
  security: SecurityConfig;
  notifications: NotificationConfig;
  server: ServerConfig;
  openai: TargetConfig;
  ollama: TargetConfig;
  [key: string]: unknown;
}

interface AppConfig {
  config: ConfigData;
  note: string;
  examples: Record<string, string>;
}

export default function Dashboard() {
  const [darkMode, setDarkMode] = useState(true);
  const [selectedRequestDetails, setSelectedRequestDetails] = useState<DetailedRequestEvent | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [appConfig, setAppConfig] = useState<AppConfig | null>(null);
  const [stats, setStats] = useState<DashboardStats>({
    totalRequests: 0,
    totalDetections: 0,
    activeDetectors: 41,
    uptime: '0h 0m',
    recentEvents: []
  });

  useEffect(() => {
    document.documentElement.classList.toggle('dark', darkMode);
  }, [darkMode]);

  useEffect(() => {
    // Fetch initial stats
    const fetchStats = async () => {
      try {
        const response = await fetch('/api/stats');
        if (response.ok) {
          const data = await response.json();
          setStats({
            totalRequests: data.totalRequests,
            totalDetections: data.recentMetrics.reduce((sum: number, metric: RecentMetric) => sum + metric.detections.length, 0),
            activeDetectors: 41,
            uptime: '0h 0m',
            recentEvents: data.recentMetrics.slice(0, 5).map((metric: RecentMetric) => ({
              id: metric.id,
              timestamp: metric.timestamp,
              entityType: metric.detections[0]?.entityType || 'unknown',
              maskedAs: metric.detections[0]?.masked || '[MASKED]',
              occurrences: metric.detections[0]?.count || 1,
              endpoint: metric.endpoint,
              provider: getProviderFromEndpoint(metric.endpoint)
            }))
          });
        }
      } catch (error) {
        console.error('Error fetching initial stats:', error);
      }
    };

    // Fetch configuration
    const fetchConfig = async () => {
      try {
        const response = await fetch('/api/config');
        if (response.ok) {
          const data = await response.json();
          setAppConfig(data);
        }
      } catch (error) {
        console.error('Error fetching config:', error);
      }
    };

    fetchStats();
    fetchConfig();

    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${wsProtocol}//${window.location.host}/ws`;
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        
        switch (message.type) {
          case 'stats':
            setStats(prev => ({
              ...prev,
              totalRequests: message.data.totalRequests || prev.totalRequests,
              uptime: `${Math.floor(message.data.uptime / 3600)}h ${Math.floor((message.data.uptime % 3600) / 60)}m`
            }));
            break;

          case 'request_event':
            if (message.data.hasDetections) {
              const newEvent: DetectionEvent = {
                id: message.data.id,
                timestamp: message.data.timestamp,
                entityType: message.data.detections[0]?.entityType || 'unknown',
                maskedAs: message.data.detections[0]?.masked || '[MASKED]',
                occurrences: message.data.detections[0]?.count || 1,
                endpoint: message.data.endpoint,
                provider: getProviderFromEndpoint(message.data.endpoint)
              };
              
              setStats(prev => {
                // Check if event with this ID already exists
                const existingEventIndex = prev.recentEvents.findIndex(event => event.id === message.data.id);
                
                let updatedEvents;
                if (existingEventIndex >= 0) {
                  // Update existing event
                  updatedEvents = [...prev.recentEvents];
                  updatedEvents[existingEventIndex] = newEvent;
                } else {
                  // Add new event and increment counters only for new events
                  updatedEvents = [newEvent, ...prev.recentEvents.slice(0, 9)];
                }
                
                return {
                  ...prev,
                  totalDetections: existingEventIndex >= 0 ? prev.totalDetections : prev.totalDetections + message.data.detections.length,
                  totalRequests: (existingEventIndex >= 0 || message.data.status !== 'processing') ? prev.totalRequests : prev.totalRequests + 1,
                  recentEvents: updatedEvents
                };
              });
            } else if (message.data.status === 'processing') {
              setStats(prev => ({
                ...prev,
                totalRequests: prev.totalRequests + 1
              }));
            }
            break;
        }
      } catch (error) {
        console.error('WebSocket message parsing error:', error);
      }
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      setTimeout(() => {
        window.location.reload();
      }, 5000);
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    return () => {
      ws.close();
    };
  }, []);

  const fetchRequestDetails = async (requestId: string) => {
    try {
      const response = await fetch(`/api/request/${requestId}`);
      if (response.ok) {
        const details = await response.json();
        setSelectedRequestDetails(details);
        setIsDetailModalOpen(true);
      } else {
        console.error('Failed to fetch request details');
      }
    } catch (error) {
      console.error('Error fetching request details:', error);
    }
  };

  const getProviderFromEndpoint = (endpoint: string): DetectionEvent['provider'] => {
    if (endpoint.includes('/openai') || endpoint.includes('/v1/chat') || endpoint.includes('/v1/completions')) {
      return 'OpenAI';
    } else if (endpoint.includes('/ollama') || endpoint.includes('/api/generate') || endpoint.includes('/api/chat')) {
      return 'Ollama';
    } else if (endpoint.includes('/cursor')) {
      return 'Cursor';
    } else if (endpoint.includes('/copilot') || endpoint.includes('/github')) {
      return 'GitHub Copilot';
    } else if (endpoint.includes('/claude') || endpoint.includes('/anthropic')) {
      return 'Claude';
    }
    return 'Unknown';
  };

  const getProviderColor = (provider: DetectionEvent['provider']) => {
    // Since we're using pure black/white theme, just return simple contrast
    return provider === 'OpenAI' || provider === 'Ollama' ? 
      'text-foreground bg-muted border-border' : 
      'text-muted-foreground bg-muted/50 border-border';
  };

  const openRequestDetails = (event: DetectionEvent) => {
    if (event.id) {
      fetchRequestDetails(event.id);
    }
  };

  // Dynamic detector categories based on real config data
  const detectorCategories = appConfig?.config?.detection?.enabledRules 
    ? [
        { name: 'API Keys', count: appConfig.config.detection.enabledRules.filter((r: string) => r.includes('Api') || r.includes('Key')).length },
        { name: 'Personal Data', count: appConfig.config.detection.enabledRules.filter((r: string) => r.includes('email') || r.includes('phone')).length },
        { name: 'Credentials', count: appConfig.config.detection.enabledRules.filter((r: string) => r.includes('aws') || r.includes('token')).length },
        { name: 'All Active', count: appConfig.config.detection.enabledRules.length }
      ]
    : [
        { name: 'Loading...', count: 0 }
      ];

  return (
    <div className="min-h-screen bg-background text-foreground p-6">
      <div className="max-w-7xl mx-auto space-y-8">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <h1 className="text-4xl font-bold text-foreground">
              🛡️ LLM-Sentinel
            </h1>
            <p className="text-muted-foreground text-lg">
              Privacy-first AI proxy • Real-time protection monitoring
            </p>
          </div>
          
          <div className="flex items-center space-x-4">
            <div className="flex items-center space-x-2">
              <Sun className="h-4 w-4" />
              <Switch checked={darkMode} onCheckedChange={setDarkMode} />
              <Moon className="h-4 w-4" />
            </div>
            <Badge variant="outline" className="neon-glow">
              <Activity className="w-4 h-4 mr-2" />
              Live
            </Badge>
          </div>
        </div>

        {/* Status Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          <Card className="glass-effect">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
              <Globe className="h-4 w-4" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.totalRequests.toLocaleString()}</div>
              <p className="text-xs text-muted-foreground">
                +12.5% from last hour
              </p>
              <Progress value={75} className="mt-2" />
            </CardContent>
          </Card>

          <Card className="glass-effect">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Detections</CardTitle>
              <AlertTriangle className="h-4 w-4" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.totalDetections}</div>
              <p className="text-xs text-muted-foreground">
                Sensitive data blocked
              </p>
              <Progress value={60} className="mt-2" />
            </CardContent>
          </Card>

          <Card className="glass-effect">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Active Detectors</CardTitle>
              <Shield className="h-4 w-4" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.activeDetectors}/41</div>
              <p className="text-xs text-muted-foreground">
                Protection enabled
              </p>
              <Progress value={100} className="mt-2" />
            </CardContent>
          </Card>

          <Card className="glass-effect">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Uptime</CardTitle>
              <Zap className="h-4 w-4" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.uptime}</div>
              <p className="text-xs text-muted-foreground">
                99.9% availability
              </p>
              <Progress value={99.9} className="mt-2" />
            </CardContent>
          </Card>
        </div>

        {/* Main Content */}
        <Tabs defaultValue="overview" className="space-y-6">
          <TabsList className="grid w-full grid-cols-4">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="detectors">Detectors</TabsTrigger>
            <TabsTrigger value="events">Live Events</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-6">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              {/* Detector Categories */}
              <Card className="glass-effect">
                <CardHeader>
                  <CardTitle className="flex items-center space-x-2">
                    <Database className="w-5 h-5 text-primary" />
                    <span>Detection Categories</span>
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  {detectorCategories.map((category, index) => (
                    <div key={index} className="flex items-center justify-between p-3 rounded-lg bg-muted/50">
                      <div className="flex items-center space-x-3">
                        <div className="w-3 h-3 rounded-full border border-foreground" />
                        <span className="font-medium">{category.name}</span>
                      </div>
                      <Badge variant="secondary">{category.count}</Badge>
                    </div>
                  ))}
                </CardContent>
              </Card>

              {/* Recent Activity */}
              <Card className="glass-effect">
                <CardHeader>
                  <CardTitle className="flex items-center space-x-2">
                    <Activity className="w-5 h-5 text-primary" />
                    <span>Recent Activity</span>
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  {stats.recentEvents.map((event) => (
                    <div 
                      key={event.id} 
                      className="flex items-start space-x-3 p-3 rounded-lg bg-muted/50 hover:bg-muted/70 cursor-pointer transition-colors"
                      onClick={() => openRequestDetails(event)}
                    >
                      <Lock className="w-4 h-4 mt-1" />
                      <div className="flex-1 space-y-1">
                        <div className="flex items-center justify-between">
                          <Badge variant="outline" className="text-xs">
                            {event.maskedAs}
                          </Badge>
                          <div className="flex items-center space-x-2">
                            <span className="text-xs text-muted-foreground">
                              {new Date(event.timestamp).toLocaleTimeString()}
                            </span>
                            <ExternalLink className="w-3 h-3 text-muted-foreground" />
                          </div>
                        </div>
                        <div className="flex items-center justify-between">
                          <p className="text-sm text-muted-foreground">
                            {event.endpoint} • {event.occurrences} occurrence{event.occurrences > 1 ? 's' : ''}
                          </p>
                          {event.provider && (
                            <Badge 
                              variant="secondary" 
                              className={`text-xs px-2 py-1 ${getProviderColor(event.provider)} dark:text-foreground dark:bg-muted`}
                            >
                              {event.provider}
                            </Badge>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="detectors" className="space-y-6">
            <Card className="glass-effect">
              <CardHeader>
                <CardTitle>All Detection Rules (41 Active)</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {[
                    'OpenAI API Key', 'Claude API Key', 'AWS Access Key', 'Email Address',
                    'Credit Card', 'SSH Private Key', 'JWT Token', 'Phone Number',
                    'PostgreSQL URL', 'MongoDB URL', 'Stripe API Key', 'GitHub Token',
                    'Docker Registry Auth', 'Slack Token', 'Discord Token', 'IP Address'
                  ].map((detector, index) => (
                    <div key={index} className="flex items-center justify-between p-3 rounded-lg bg-muted/30">
                      <span className="text-sm">{detector}</span>
                      <Badge variant="outline" className="text-xs">
                        Active
                      </Badge>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="events" className="space-y-6">
            <Card className="glass-effect">
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <span>Live Detection Events</span>
                  <Badge variant="outline" className="animate-pulse-slow">
                    <Eye className="w-3 h-3 mr-1" />
                    Monitoring
                  </Badge>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {stats.recentEvents.map((event) => (
                    <div key={event.id} className="flex items-center justify-between p-4 rounded-lg bg-muted/30 border-l-4 border-foreground">
                      <div>
                        <div className="font-medium">{event.maskedAs}</div>
                        <div className="text-sm text-muted-foreground">
                          {event.endpoint} • {new Date(event.timestamp).toLocaleString()}
                        </div>
                      </div>
                      <Badge variant="outline">{event.occurrences}</Badge>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="settings" className="space-y-6">
            <Card className="glass-effect">
              <CardHeader>
                <CardTitle className="flex items-center space-x-2">
                  <Database className="w-5 h-5 text-primary" />
                  <span>Complete Configuration</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                {appConfig ? (
                  <div className="space-y-6">
                    {/* Instructions */}
                    <div className="bg-muted/50 border-l-4 border-primary p-4 rounded">
                      <p className="text-sm font-medium mb-2">{appConfig.note}</p>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                        {Object.entries(appConfig.examples).map(([description, command]) => (
                          <div key={description} className="text-xs">
                            <span className="text-muted-foreground">{description}:</span>
                            <br />
                            <code className="bg-background px-2 py-1 rounded">{command as string}</code>
                          </div>
                        ))}
                      </div>
                    </div>

                    <Separator />

                    {/* Current Configuration JSON */}
                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <h3 className="text-lg font-semibold">Current Configuration</h3>
                        <Badge variant="outline" className="text-xs">
                          Read-Only
                        </Badge>
                      </div>
                      
                      <ScrollArea className="h-96 w-full rounded border bg-muted/30 p-4">
                        <pre className="text-xs whitespace-pre-wrap">
                          {JSON.stringify(appConfig.config, null, 2)}
                        </pre>
                      </ScrollArea>
                    </div>

                    {/* Key Configuration Summary */}
                    <div className="space-y-4">
                      <h3 className="text-lg font-semibold">Configuration Summary</h3>
                      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                        
                        {/* Detection Settings */}
                        <div className="space-y-2">
                          <h4 className="font-medium flex items-center space-x-2">
                            <Shield className="w-4 h-4" />
                            <span>Detection</span>
                          </h4>
                          <div className="space-y-1 text-sm">
                            <div className="flex justify-between">
                              <span>Enabled:</span>
                              <Badge variant={appConfig.config.detection?.enabled ? "default" : "destructive"}>
                                {appConfig.config.detection?.enabled ? "Yes" : "No"}
                              </Badge>
                            </div>
                            {appConfig.config.detection?.enabledRules && (
                              <div className="flex justify-between">
                                <span>Active Rules:</span>
                                <Badge variant="secondary">
                                  {appConfig.config.detection.enabledRules.length}
                                </Badge>
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Logging Settings */}
                        <div className="space-y-2">
                          <h4 className="font-medium flex items-center space-x-2">
                            <Activity className="w-4 h-4" />
                            <span>Logging</span>
                          </h4>
                          <div className="space-y-1 text-sm">
                            <div className="flex justify-between">
                              <span>Level:</span>
                              <Badge variant="outline">
                                {appConfig.config.logging?.logLevel || "INFO"}
                              </Badge>
                            </div>
                            <div className="flex justify-between">
                              <span>To File:</span>
                              <Badge variant={appConfig.config.logging?.logToFile ? "default" : "destructive"}>
                                {appConfig.config.logging?.logToFile ? "Yes" : "No"}
                              </Badge>
                            </div>
                            <div className="flex justify-between">
                              <span>Console:</span>
                              <Badge variant={appConfig.config.logging?.logToConsole ? "default" : "destructive"}>
                                {appConfig.config.logging?.logToConsole ? "Yes" : "No"}
                              </Badge>
                            </div>
                          </div>
                        </div>

                        {/* Security Settings */}
                        <div className="space-y-2">
                          <h4 className="font-medium flex items-center space-x-2">
                            <Lock className="w-4 h-4" />
                            <span>Security</span>
                          </h4>
                          <div className="space-y-1 text-sm">
                            <div className="flex justify-between">
                              <span>Redact API Keys:</span>
                              <Badge variant={appConfig.config.security?.redactApiKeys ? "default" : "destructive"}>
                                {appConfig.config.security?.redactApiKeys ? "Yes" : "No"}
                              </Badge>
                            </div>
                            {appConfig.config.security?.redactCustomHeaders && (
                              <div className="flex justify-between">
                                <span>Custom Headers:</span>
                                <Badge variant="secondary">
                                  {appConfig.config.security.redactCustomHeaders.length}
                                </Badge>
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Notification Settings */}
                        <div className="space-y-2">
                          <h4 className="font-medium flex items-center space-x-2">
                            <AlertTriangle className="w-4 h-4" />
                            <span>Notifications</span>
                          </h4>
                          <div className="space-y-1 text-sm">
                            <div className="flex justify-between">
                              <span>Enabled:</span>
                              <Badge variant={appConfig.config.notifications?.enabled ? "default" : "destructive"}>
                                {appConfig.config.notifications?.enabled ? "Yes" : "No"}
                              </Badge>
                            </div>
                            <div className="flex justify-between">
                              <span>Sound:</span>
                              <Badge variant={appConfig.config.notifications?.sound ? "default" : "destructive"}>
                                {appConfig.config.notifications?.sound ? "Yes" : "No"}
                              </Badge>
                            </div>
                          </div>
                        </div>

                        {/* Server Settings */}
                        <div className="space-y-2">
                          <h4 className="font-medium flex items-center space-x-2">
                            <Server className="w-4 h-4" />
                            <span>Server</span>
                          </h4>
                          <div className="space-y-1 text-sm">
                            <div className="flex justify-between">
                              <span>Port:</span>
                              <Badge variant="outline">
                                {appConfig.config.server?.port || 5050}
                              </Badge>
                            </div>
                            <div className="flex justify-between">
                              <span>OpenAI:</span>
                              <Badge variant="outline" className="text-xs">
                                {appConfig.config.openai?.target || "api.openai.com"}
                              </Badge>
                            </div>
                            <div className="flex justify-between">
                              <span>Ollama:</span>
                              <Badge variant="outline" className="text-xs">
                                {appConfig.config.ollama?.target || "localhost:11434"}
                              </Badge>
                            </div>
                          </div>
                        </div>

                        {/* Rules Summary */}
                        {appConfig.config.detection?.enabledRules && (
                          <div className="space-y-2">
                            <h4 className="font-medium flex items-center space-x-2">
                              <Eye className="w-4 h-4" />
                              <span>Detection Rules</span>
                            </h4>
                            <ScrollArea className="h-32">
                              <div className="space-y-1">
                                {appConfig.config.detection.enabledRules.map((rule: string, index: number) => (
                                  <Badge key={index} variant="outline" className="text-xs mr-1 mb-1">
                                    {rule}
                                  </Badge>
                                ))}
                              </div>
                            </ScrollArea>
                          </div>
                        )}

                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="text-center py-8">
                    <div className="animate-pulse">Loading configuration...</div>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        {/* Footer */}
        <div className="text-center text-sm text-muted-foreground pt-8 border-t border-border/50">
          <p>LLM-Sentinel • Built with ❤️ by <a href="https://github.com/raaihank" className="text-primary hover:text-primary/80">Raihan K.</a></p>
        </div>
      </div>

      {/* Detailed Request Modal */}
      <Dialog open={isDetailModalOpen} onOpenChange={setIsDetailModalOpen}>
        <DialogContent className="max-w-5xl w-[95vw] max-h-[95vh] overflow-hidden border-2 border-foreground/50 shadow-xl shadow-foreground/20 bg-background/95 backdrop-blur-sm">
          <DialogHeader className="pb-4">
            <DialogTitle className="flex items-center space-x-2">
              <Server className="w-5 h-5" />
              <span>Request Details</span>
              {selectedRequestDetails && (
                <>
                  <Badge variant="outline" className="ml-2">
                    {selectedRequestDetails.status}
                  </Badge>
                  <Badge 
                    variant="secondary" 
                    className={`text-xs px-2 py-1 ${getProviderColor(getProviderFromEndpoint(selectedRequestDetails.endpoint))} dark:text-foreground dark:bg-muted`}
                  >
                    {getProviderFromEndpoint(selectedRequestDetails.endpoint)}
                  </Badge>
                </>
              )}
            </DialogTitle>
            <DialogDescription>
              Detailed information about the intercepted request and response
            </DialogDescription>
          </DialogHeader>
          
          {selectedRequestDetails && (
            <ScrollArea className="max-h-[75vh] pr-4">
              <div className="space-y-6 pb-4">
                {/* Overview */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <div className="space-y-1">
                    <div className="text-sm font-medium">Method</div>
                    <Badge variant="secondary">{selectedRequestDetails.method}</Badge>
                  </div>
                  <div className="space-y-1">
                    <div className="text-sm font-medium">Status</div>
                    <Badge variant={selectedRequestDetails.responseStatus === 200 ? "default" : "destructive"}>
                      {selectedRequestDetails.responseStatus || 'N/A'}
                    </Badge>
                  </div>
                  <div className="space-y-1">
                    <div className="text-sm font-medium flex items-center space-x-1">
                      <Clock className="w-3 h-3" />
                      <span>Processing</span>
                    </div>
                    <div className="text-sm">{selectedRequestDetails.processingTimeMs}ms</div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-sm font-medium flex items-center space-x-1">
                      <Clock className="w-3 h-3" />
                      <span>API Response</span>
                    </div>
                    <div className="text-sm">{selectedRequestDetails.apiResponseTimeMs || 0}ms</div>
                  </div>
                </div>

                <Separator />

                {/* Detections */}
                {selectedRequestDetails.detections && selectedRequestDetails.detections.length > 0 && (
                  <>
                    <div className="space-y-3">
                      <h3 className="text-lg font-semibold flex items-center space-x-2">
                        <AlertTriangle className="w-5 h-5 text-primary" />
                        <span>Sensitive Data Detections</span>
                      </h3>
                      <div className="grid gap-2">
                        {selectedRequestDetails.detections.map((detection, index) => (
                          <div key={index} className="flex items-center justify-between p-3 rounded-lg bg-slate-50 dark:bg-slate-700">
                            <div className="flex items-center space-x-3">
                              <Lock className="w-4 h-4" />
                              <span className="font-medium">{detection.entityType}</span>
                            </div>
                            <div className="flex items-center space-x-2">
                              <Badge variant="outline">{detection.masked}</Badge>
                              <span className="text-sm text-muted-foreground">×{detection.count}</span>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                    <Separator />
                  </>
                )}

                {/* Request Details */}
                <div className="space-y-4">
                  <h3 className="text-lg font-semibold">Request</h3>
                  <div className="space-y-3">
                    <div>
                      <div className="text-sm font-medium mb-2">Endpoint</div>
                      <code className="text-sm bg-slate-100 dark:bg-slate-800 p-2 rounded block">
                        {selectedRequestDetails.method} {selectedRequestDetails.endpoint}
                      </code>
                    </div>
                    
                    {selectedRequestDetails.requestHeaders && (
                      <div>
                        <div className="text-sm font-medium mb-2">Headers</div>
                        <ScrollArea className="h-32">
                          <pre className="text-xs bg-slate-100 dark:bg-slate-800 p-3 rounded whitespace-pre overflow-x-auto">
                            {JSON.stringify(selectedRequestDetails.requestHeaders, null, 4)}
                          </pre>
                        </ScrollArea>
                      </div>
                    )}

                    {selectedRequestDetails.requestBody && (
                      <div>
                        <div className="text-sm font-medium mb-2 flex items-center space-x-2">
                          <span>Original Request Body</span>
                          <Badge variant="outline" className="text-xs">
                            {selectedRequestDetails.contentLength} bytes
                          </Badge>
                        </div>
                        <ScrollArea className="h-40">
                          <pre className="text-xs bg-slate-100 dark:bg-slate-800 p-3 rounded whitespace-pre overflow-x-auto">
                            {selectedRequestDetails.requestBody}
                          </pre>
                        </ScrollArea>
                      </div>
                    )}

                    {selectedRequestDetails.maskedRequestBody && selectedRequestDetails.maskedRequestBody !== selectedRequestDetails.requestBody && (
                      <div>
                        <div className="text-sm font-medium mb-2 flex items-center space-x-2">
                          <span>Masked Request Body (Sent to AI)</span>
                          <Badge variant="secondary" className="text-xs">MASKED</Badge>
                        </div>
                        <ScrollArea className="h-40">
                          <pre className="text-xs bg-slate-100 dark:bg-slate-800 p-3 rounded whitespace-pre overflow-x-auto">
                            {selectedRequestDetails.maskedRequestBody}
                          </pre>
                        </ScrollArea>
                      </div>
                    )}
                  </div>
                </div>

                <Separator />

                {/* Response Details */}
                {selectedRequestDetails.responseBody && (
                  <div className="space-y-4">
                    <h3 className="text-lg font-semibold">Response</h3>
                    <div className="space-y-3">
                      {selectedRequestDetails.responseHeaders && (
                        <div>
                          <div className="text-sm font-medium mb-2">Response Headers</div>
                          <ScrollArea className="h-32">
                            <pre className="text-xs bg-slate-100 dark:bg-slate-800 p-3 rounded whitespace-pre overflow-x-auto">
                              {JSON.stringify(selectedRequestDetails.responseHeaders, null, 4)}
                            </pre>
                          </ScrollArea>
                        </div>
                      )}
                      
                      <div>
                        <div className="text-sm font-medium mb-2">Response Body</div>
                        <ScrollArea className="h-48">
                          <pre className="text-xs bg-slate-100 dark:bg-slate-800 p-3 rounded whitespace-pre overflow-x-auto">
                            {selectedRequestDetails.responseBody}
                          </pre>
                        </ScrollArea>
                      </div>
                    </div>
                  </div>
                )}

                {/* Logs */}
                {selectedRequestDetails.logs && selectedRequestDetails.logs.length > 0 && (
                  <>
                    <Separator />
                    <div className="space-y-4">
                      <h3 className="text-lg font-semibold">Processing Logs</h3>
                      <div className="space-y-2">
                        {selectedRequestDetails.logs.map((log, index) => (
                          <div key={index} className="flex items-start space-x-3 p-3 rounded-lg bg-slate-50 dark:bg-slate-700">
                            <div className="text-xs text-muted-foreground mt-1">
                              {new Date(log.timestamp).toLocaleTimeString()}
                            </div>
                            <div className="flex-1">
                              <div className="font-medium text-sm">{log.message}</div>
                              {log.data && (
                                <div className="text-xs text-muted-foreground mt-1">
                                  <pre className="whitespace-pre overflow-x-auto bg-slate-100 dark:bg-slate-800 p-2 rounded">
                                    {JSON.stringify(log.data, null, 4)}
                                  </pre>
                                </div>
                              )}
                            </div>
                            <Badge variant="outline" className="text-xs">
                              {log.level}
                            </Badge>
                          </div>
                        ))}
                      </div>
                    </div>
                  </>
                )}
              </div>
            </ScrollArea>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}