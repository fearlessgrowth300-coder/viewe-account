import React, { useState, useEffect, useRef } from 'react';
import {
  LayoutDashboard,
  Tv,
  Layers,
  MessageSquare,
  BarChart3,
  CreditCard,
  Settings,
  Code2,
  HelpCircle,
  LogOut,
  Search,
  Bell,
  Globe,
  Plus,
  ChevronDown,
  Eye,
  Zap,
  Play,
  Square,
  Sliders,
  Radio,
  CheckCircle2,
  Clock,
  Sparkles,
  RefreshCw,
  Send,
  MoreVertical,
  X,
  Flame,
  ShieldAlert,
  UserCheck,
  Unlock,
  Bot,
  Wifi,
  Cpu
} from 'lucide-react';

export default function DashboardLayout() {
  const [activeTab, setActiveTab] = useState('Dashboard');
  const [activeModal, setActiveModal] = useState(null);
  
  // Customizable Configuration
  const [viewerCount, setViewerCount] = useState(100);
  const [activeChatters, setActiveChatters] = useState(12);
  const [chatFrequency, setChatFrequency] = useState(10);
  const [disableViewerlist, setDisableViewerlist] = useState(false);
  const [autoFollow, setAutoFollow] = useState(true);
  const [autoUnlockChat, setAutoUnlockChat] = useState(true);
  const [useAIChat, setUseAIChat] = useState(true); // AI Contextual Chat Toggle
  const [selectedPlatform, setSelectedPlatform] = useState('twitch');
  const [channelName, setChannelName] = useState('zaybosays');
  const [isStarting, setIsStarting] = useState(false);
  const [wsConnected, setWsConnected] = useState(false);

  // Live Telemetry Stats
  const [stats, setStats] = useState({
    active_viewers: 100,
    total_chatters: 12,
    plans_running: 1,
    proxy_bandwidth_mb: 84.2,
    chat_messages_per_min: 24,
    system_telemetry: { cpu_percent: 14.2, ram_percent: 22.4 }
  });

  // Live Real-Time Chat Stream (Updated via WebSocket)
  const [chatMessages, setChatMessages] = useState([
    { id: 1, user: 'boy_mular3', bot: true, isAI: true, text: 'that movement was clean 🔥', time: '14:40' },
    { id: 2, user: 'stream_fan99', bot: false, text: 'what weapon is that?', time: '14:40' },
    { id: 3, user: 'boy_mular3', bot: true, isAI: true, text: 'actual hype moment', time: '14:41' },
    { id: 4, user: 'chaangame', bot: false, text: 'insane reaction time', time: '14:41' },
  ]);

  const wsRef = useRef(null);

  // ============================================================================
  // PERSISTENT WEBSOCKET CONNECTION (Real-time events without page refresh)
  // ============================================================================
  useEffect(() => {
    const connectWebSocket = () => {
      const ws = new WebSocket('ws://127.0.0.1:8000/ws/live');
      wsRef.current = ws;

      ws.onopen = () => {
        setWsConnected(true);
        console.log('[WS] Persistent connection established with backend.');
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === 'chat_message') {
            setChatMessages((prev) => [...prev.slice(-30), data.payload]);
          } else if (data.type === 'telemetry_update') {
            setStats(data.payload);
          }
        } catch (e) {
          console.error('[WS Parse Error]', e);
        }
      };

      ws.onclose = () => {
        setWsConnected(false);
        setTimeout(connectWebSocket, 3000); // Auto-reconnect
      };
    };

    connectWebSocket();
    return () => wsRef.current?.close();
  }, []);

  const handleStartTask = async () => {
    setIsStarting(true);
    try {
      const payload = {
        channel_name: channelName,
        platform: selectedPlatform === 'twitch' ? 'Twitch' : 'Kick',
        viewer_count: Number(viewerCount),
        use_chat: true,
        use_ai_llm_chat: useAIChat,
        auto_follow: autoFollow,
        auto_unlock_chat: autoUnlockChat,
        proxy_tier: 'Residential'
      };

      const res = await fetch('http://127.0.0.1:8000/api/tasks/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await res.json();
      alert(`Swarm Started: ${data.message}`);
    } catch (e) {
      alert(`Failed starting task: ${e.message}`);
    } finally {
      setIsStarting(false);
      setActiveModal(null);
    }
  };

  const navItems = [
    { name: 'Dashboard', icon: LayoutDashboard },
    { name: 'My Channels', icon: Tv },
    { name: 'Plans', icon: Layers },
    { name: 'Chat Lists', icon: MessageSquare },
    { name: 'Statistics', icon: BarChart3 },
    { name: 'Billing', icon: CreditCard },
    { name: 'Settings', icon: Settings },
    { name: 'API & Integrations', icon: Code2 },
    { name: 'Support', icon: HelpCircle },
  ];

  return (
    <div className="flex h-screen w-full bg-[#0B0E14] text-slate-200 font-sans overflow-hidden select-none">
      
      {/* ========================================================================= */}
      {/* 1. LEFT SIDEBAR NAVIGATION                                               */}
      {/* ========================================================================= */}
      <aside className="w-64 flex flex-col justify-between border-r border-[#1E2638] bg-[#0E131F]/90 p-4 shrink-0">
        <div className="space-y-6">
          <div className="flex items-center gap-3 px-2 py-1">
            <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-[#6366F1] to-[#9333EA] flex items-center justify-center shadow-lg shadow-[#6366F1]/30">
              <Sparkles className="w-5 h-5 text-white" />
            </div>
            <span className="text-lg font-black tracking-wider text-white uppercase">VIEWBOTTER</span>
          </div>

          <nav className="space-y-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = activeTab === item.name;
              return (
                <button
                  key={item.name}
                  onClick={() => setActiveTab(item.name)}
                  className={`w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium transition-all duration-150 ${
                    isActive
                      ? 'bg-[#161B26] text-white border-l-4 border-[#6366F1] shadow-md shadow-[#6366F1]/10 font-semibold'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-[#161B26]/50'
                  }`}
                >
                  <Icon className={`w-4 h-4 ${isActive ? 'text-[#6366F1]' : 'text-slate-400'}`} />
                  {item.name}
                </button>
              );
            })}
          </nav>
        </div>

        {/* WebSocket Connection Indicator */}
        <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-4 space-y-2">
          <div className="flex items-center justify-between text-xs">
            <span className="font-semibold text-slate-300">Live Socket</span>
            <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold ${
              wsConnected ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-400'
            }`}>
              <span className={`w-1.5 h-1.5 rounded-full ${wsConnected ? 'bg-emerald-400 animate-pulse' : 'bg-rose-400'}`} />
              {wsConnected ? 'Connected' : 'Offline'}
            </span>
          </div>
          <p className="text-[11px] text-slate-500">Real-time WebSocket streaming</p>
        </div>
      </aside>

      {/* ========================================================================= */}
      {/* 2. MAIN DASHBOARD CONTENT AREA                                            */}
      {/* ========================================================================= */}
      <div className="flex-1 flex flex-col h-screen overflow-y-auto">
        
        {/* Top Header Bar */}
        <header className="h-16 border-b border-[#1E2638] bg-[#0E131F]/40 backdrop-blur-md px-8 flex items-center justify-between sticky top-0 z-30">
          <div className="flex items-center gap-4 flex-1 max-w-md">
            <div className="relative w-full">
              <Search className="w-4 h-4 text-slate-500 absolute left-3.5 top-3" />
              <input
                type="text"
                placeholder="Search channels, tasks, proxies..."
                className="w-full bg-[#161B26] border border-[#1E2638] rounded-xl pl-10 pr-4 py-2 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-[#6366F1]"
              />
            </div>
          </div>

          <div className="flex items-center gap-4">
            <button className="p-2 rounded-xl bg-[#161B26] border border-[#1E2638] text-slate-400 hover:text-white">
              <Globe className="w-4 h-4" />
            </button>
            <button className="p-2 rounded-xl bg-[#161B26] border border-[#1E2638] text-slate-400 hover:text-white relative">
              <Bell className="w-4 h-4" />
              <span className="w-2 h-2 rounded-full bg-[#6366F1] absolute top-1.5 right-1.5" />
            </button>
            <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-[#6366F1] to-[#9333EA] text-white flex items-center justify-center text-xs font-bold shadow-md">
              TA
            </div>
          </div>
        </header>

        {/* Dashboard Canvas */}
        <main className="p-8 space-y-8 flex-1 max-w-7xl w-full mx-auto">
          
          {/* Header */}
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-black text-white tracking-tight">Automation Engine</h1>
              <p className="text-xs text-slate-400 mt-1">Configure multi-instance concurrency, stealth fingerprints, and AI chatters</p>
            </div>
            <div className="flex items-center gap-3">
              <button 
                onClick={() => setActiveModal('manage-plan')}
                className="px-4 py-2 rounded-xl bg-[#161B26] hover:bg-[#1E2638] text-slate-200 border border-[#1E2638] text-xs font-semibold flex items-center gap-2 cursor-pointer"
              >
                <Sliders className="w-4 h-4 text-slate-400" /> Instance Settings
              </button>
              <button 
                onClick={() => setActiveModal('add-channel')}
                className="px-4 py-2 rounded-xl bg-[#6366F1] hover:bg-[#4F46E5] text-white text-xs font-bold shadow-lg shadow-[#6366F1]/25 flex items-center gap-2 cursor-pointer"
              >
                <Plus className="w-4 h-4" /> Launch Swarm
              </button>
            </div>
          </div>

          {/* Metric Stats */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-6 flex items-center justify-between shadow-sm relative overflow-hidden">
              <div className="absolute top-0 left-0 h-full w-1 bg-[#10B981]" />
              <div>
                <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider">TOTAL INSTANCES</p>
                <p className="text-3xl font-black text-white mt-2">{viewerCount}</p>
              </div>
              <div className="w-12 h-12 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 flex items-center justify-center">
                <Eye className="w-6 h-6" />
              </div>
            </div>

            <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-6 flex items-center justify-between shadow-sm relative overflow-hidden">
              <div className="absolute top-0 left-0 h-full w-1 bg-[#3B82F6]" />
              <div>
                <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider">AI CHATTERS ACTIVE</p>
                <p className="text-3xl font-black text-white mt-2">{activeChatters}</p>
              </div>
              <div className="w-12 h-12 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-400 flex items-center justify-center">
                <Bot className="w-6 h-6" />
              </div>
            </div>

            <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-6 flex items-center justify-between shadow-sm relative overflow-hidden">
              <div className="absolute top-0 left-0 h-full w-1 bg-[#6366F1]" />
              <div>
                <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider">PROXY BANDWIDTH</p>
                <p className="text-3xl font-black text-white mt-2">{stats.proxy_bandwidth_mb} MB</p>
              </div>
              <div className="w-12 h-12 rounded-xl bg-[#6366F1]/10 border border-[#6366F1]/20 text-[#6366F1] flex items-center justify-center">
                <Wifi className="w-6 h-6" />
              </div>
            </div>
          </div>

          {/* AI Stream Intelligence & Stealth Controls */}
          <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-6 space-y-6">
            <div className="flex items-center justify-between border-b border-[#1E2638] pb-4">
              <div>
                <h2 className="text-base font-bold text-white flex items-center gap-2">
                  <Bot className="w-5 h-5 text-[#6366F1]" /> AI LLM Stream Scanner & Fingerprint Engine
                </h2>
                <p className="text-xs text-slate-400 mt-0.5">Advanced contextual scanning and unique hardware profile rotation</p>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {/* AI Chat Mode Toggle */}
              <div className="p-4 rounded-xl bg-[#0D111A] border border-[#1E2638] flex items-center justify-between">
                <div>
                  <p className="text-xs font-bold text-white flex items-center gap-1.5">
                    <Sparkles className="w-4 h-4 text-purple-400" /> AI Context Scanner
                  </p>
                  <p className="text-[10px] text-slate-500">Scans live chat & generates dynamic replies</p>
                </div>
                <button
                  onClick={() => setUseAIChat(!useAIChat)}
                  className={`w-11 h-6 rounded-full p-0.5 transition-colors cursor-pointer ${useAIChat ? 'bg-[#6366F1]' : 'bg-slate-700'}`}
                >
                  <div className={`w-5 h-5 rounded-full bg-white transition-transform ${useAIChat ? 'translate-x-5' : 'translate-x-0'}`} />
                </button>
              </div>

              {/* Auto-Follow Toggle */}
              <div className="p-4 rounded-xl bg-[#0D111A] border border-[#1E2638] flex items-center justify-between">
                <div>
                  <p className="text-xs font-bold text-white flex items-center gap-1.5">
                    <UserCheck className="w-4 h-4 text-emerald-400" /> Auto-Follow Channel
                  </p>
                  <p className="text-[10px] text-slate-500">Unlocks follower-only chat privileges</p>
                </div>
                <button
                  onClick={() => setAutoFollow(!autoFollow)}
                  className={`w-11 h-6 rounded-full p-0.5 transition-colors cursor-pointer ${autoFollow ? 'bg-[#6366F1]' : 'bg-slate-700'}`}
                >
                  <div className={`w-5 h-5 rounded-full bg-white transition-transform ${autoFollow ? 'translate-x-5' : 'translate-x-0'}`} />
                </button>
              </div>

              {/* Auto-Unlock Chat Rules */}
              <div className="p-4 rounded-xl bg-[#0D111A] border border-[#1E2638] flex items-center justify-between">
                <div>
                  <p className="text-xs font-bold text-white flex items-center gap-1.5">
                    <ShieldAlert className="w-4 h-4 text-cyan-400" /> Auto-Dismiss Rules
                  </p>
                  <p className="text-[10px] text-slate-500">Bypasses verification & mature popups</p>
                </div>
                <button
                  onClick={() => setAutoUnlockChat(!autoUnlockChat)}
                  className={`w-11 h-6 rounded-full p-0.5 transition-colors cursor-pointer ${autoUnlockChat ? 'bg-[#6366F1]' : 'bg-slate-700'}`}
                >
                  <div className={`w-5 h-5 rounded-full bg-white transition-transform ${autoUnlockChat ? 'translate-x-5' : 'translate-x-0'}`} />
                </button>
              </div>
            </div>
          </div>

          {/* Active Channel Card */}
          <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-6 space-y-6">
            <div className="bg-[#0D111A] border border-[#1E2638] rounded-2xl p-5 flex flex-col md:flex-row md:items-center justify-between gap-6">
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 rounded-xl bg-[#9146FF]/15 border border-[#9146FF]/30 flex items-center justify-center text-[#9146FF]">
                  <MessageSquare className="w-6 h-6" />
                </div>
                <div>
                  <div className="flex items-center gap-3">
                    <h3 className="text-base font-bold text-white">{channelName}</h3>
                    <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 text-[10px] font-bold">
                      ● Active Swarm
                    </span>
                  </div>
                  <p className="text-xs text-slate-500 mt-0.5">twitch.tv/{channelName}</p>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-8 text-left">
                <div>
                  <p className="text-[10px] font-semibold text-slate-500 uppercase">INSTANCES</p>
                  <p className="text-xs font-bold text-white mt-1">{viewerCount}</p>
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-slate-500 uppercase">AI CHAT</p>
                  <p className="text-xs font-bold text-purple-400 mt-1">{useAIChat ? 'Active (GPT-4o)' : 'Standard'}</p>
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-slate-500 uppercase">PROXY POOL</p>
                  <p className="text-xs font-bold text-emerald-400 mt-1">Dedicated Rotator</p>
                </div>
              </div>

              <div className="flex items-center gap-3">
                <button
                  onClick={() => setActiveModal('manage-plan')}
                  className="px-4 py-2 rounded-xl bg-[#161B26] hover:bg-[#1E2638] text-slate-200 border border-[#1E2638] text-xs font-semibold flex items-center gap-2 cursor-pointer"
                >
                  <Settings className="w-3.5 h-3.5 text-slate-400" /> Configure
                </button>
                <button
                  onClick={() => setActiveModal('live-chat')}
                  className="px-4 py-2 rounded-xl bg-[#6366F1]/10 hover:bg-[#6366F1]/20 text-[#6366F1] border border-[#6366F1]/30 text-xs font-semibold flex items-center gap-2 cursor-pointer"
                >
                  <MessageSquare className="w-3.5 h-3.5" /> Live Chat Feed
                </button>
              </div>
            </div>
          </div>

        </main>
      </div>

      {/* ========================================================================= */}
      {/* 3. MODAL: LAUNCH SWARM WITH CUSTOM INSTANCE INPUT                        */}
      {/* ========================================================================= */}
      {activeModal === 'add-channel' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="w-full max-w-md bg-[#161B26] border border-[#1E2638] rounded-2xl shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between px-6 py-4 border-b border-[#1E2638]">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Plus className="w-4 h-4 text-[#6366F1]" /> Launch Custom Swarm
              </h3>
              <button onClick={() => setActiveModal(null)} className="text-slate-400 hover:text-white p-1 cursor-pointer">
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="p-6 space-y-4">
              <div>
                <label className="text-[11px] font-bold text-slate-400 uppercase tracking-wider block mb-2">Target Channel</label>
                <input
                  type="text"
                  value={channelName}
                  onChange={(e) => setChannelName(e.target.value)}
                  placeholder="zaybosays"
                  className="w-full bg-[#0D111A] border border-[#1E2638] rounded-xl px-4 py-2.5 text-xs text-white focus:outline-none focus:border-[#6366F1]"
                />
              </div>

              <div>
                <label className="text-[11px] font-bold text-slate-400 uppercase tracking-wider block mb-2">Instance Count</label>
                <input
                  type="number"
                  min="1"
                  max="500000"
                  value={viewerCount}
                  onChange={(e) => setViewerCount(Number(e.target.value))}
                  className="w-full bg-[#0D111A] border border-[#1E2638] rounded-xl px-4 py-2.5 text-xs text-white focus:outline-none focus:border-[#6366F1]"
                />
                <p className="text-[10px] text-slate-500 mt-1">Unlimited custom instance allocation</p>
              </div>

              <button 
                onClick={handleStartTask}
                disabled={isStarting}
                className="w-full py-2.5 rounded-xl bg-[#6366F1] hover:bg-[#4F46E5] text-white text-xs font-bold shadow-lg shadow-[#6366F1]/25 mt-2 cursor-pointer"
              >
                {isStarting ? 'Initializing Background Swarm...' : '🚀 Launch Swarm'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* 4. MODAL: LIVE CHAT PREVIEW STREAM                                       */}
      {/* ========================================================================= */}
      {activeModal === 'live-chat' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="w-full max-w-lg bg-[#161B26] border border-[#1E2638] rounded-2xl shadow-2xl overflow-hidden flex flex-col h-[520px]">
            <div className="flex items-center justify-between px-6 py-4 border-b border-[#1E2638]">
              <div className="flex items-center gap-3">
                <h3 className="text-sm font-bold text-white flex items-center gap-2">
                  <MessageSquare className="w-4 h-4 text-[#6366F1]" /> Live Stream Chat Feed
                </h3>
                <span className="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 text-[10px] font-bold">● Live WebSocket</span>
              </div>
              <button onClick={() => setActiveModal(null)} className="text-slate-400 hover:text-white p-1 cursor-pointer">
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="flex-1 p-4 overflow-y-auto space-y-2.5 bg-[#0D111A] font-mono text-xs">
              {chatMessages.map((msg, idx) => (
                <div key={idx} className="flex items-start gap-2 p-1.5 rounded hover:bg-[#161B26]/60">
                  <span className="text-[10px] text-slate-500">{msg.time}</span>
                  {msg.isAI && <span className="px-1 py-0.2 rounded bg-purple-500/20 text-purple-400 text-[9px] font-extrabold">AI-LLM</span>}
                  <span className="font-bold text-purple-400">{msg.user}:</span>
                  <span className="text-slate-200">{msg.text}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

    </div>
  );
}
