import React, { useState, useEffect } from 'react';
import { Activity, Cpu, HardDrive, Wifi, MessageSquare, Users, Zap, RefreshCw } from 'lucide-react';

export const AnalyticsPanel = () => {
  const [stats, setStats] = useState({
    active_viewers: 25,
    total_chatters: 8,
    plans_running: 1,
    proxy_bandwidth_mb: 42.6,
    chat_messages_per_min: 14,
    system_telemetry: {
      cpu_percent: 18.5,
      ram_used_mb: 3200,
      ram_total_mb: 16384,
      ram_percent: 19.5
    }
  });

  const [loading, setLoading] = useState(false);

  const fetchStats = async () => {
    try {
      setLoading(true);
      const res = await fetch('http://127.0.0.1:8000/api/dashboard/stats');
      if (res.ok) {
        const data = await res.json();
        setStats(data);
      }
    } catch (err) {
      console.warn('Live telemetry fetch failed, showing fallback state');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-6 space-y-6 text-slate-200">
      <div className="flex items-center justify-between border-b border-[#1E2638] pb-4">
        <div>
          <h2 className="text-base font-bold text-white flex items-center gap-2">
            <Activity className="w-5 h-5 text-[#6366F1]" /> Live System Telemetry
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">Real-time resource and network consumption</p>
        </div>
        <button
          onClick={fetchStats}
          className="p-2 rounded-xl bg-[#0D111A] border border-[#1E2638] hover:border-[#6366F1] text-slate-400 hover:text-white transition-all"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin text-[#6366F1]' : ''}`} />
        </button>
      </div>

      {/* Telemetry Metric Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {/* CPU Gauge */}
        <div className="p-4 rounded-xl bg-[#0D111A] border border-[#1E2638] space-y-2">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span className="flex items-center gap-1.5"><Cpu className="w-3.5 h-3.5 text-cyan-400" /> CPU Load</span>
            <span className="font-bold text-white">{stats.system_telemetry.cpu_percent}%</span>
          </div>
          <div className="w-full bg-[#161B26] h-1.5 rounded-full overflow-hidden">
            <div
              className="bg-cyan-400 h-full rounded-full transition-all duration-500"
              style={{ width: `${Math.min(100, stats.system_telemetry.cpu_percent)}%` }}
            />
          </div>
        </div>

        {/* RAM Gauge */}
        <div className="p-4 rounded-xl bg-[#0D111A] border border-[#1E2638] space-y-2">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span className="flex items-center gap-1.5"><HardDrive className="w-3.5 h-3.5 text-purple-400" /> RAM Usage</span>
            <span className="font-bold text-white">{stats.system_telemetry.ram_percent}%</span>
          </div>
          <div className="w-full bg-[#161B26] h-1.5 rounded-full overflow-hidden">
            <div
              className="bg-purple-500 h-full rounded-full transition-all duration-500"
              style={{ width: `${Math.min(100, stats.system_telemetry.ram_percent)}%` }}
            />
          </div>
        </div>

        {/* Bandwidth Transfer */}
        <div className="p-4 rounded-xl bg-[#0D111A] border border-[#1E2638] space-y-2">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span className="flex items-center gap-1.5"><Wifi className="w-3.5 h-3.5 text-emerald-400" /> Bandwidth</span>
            <span className="font-bold text-emerald-400">{stats.proxy_bandwidth_mb} MB</span>
          </div>
          <p className="text-[10px] text-slate-500">Resource blocking active</p>
        </div>

        {/* Chat Pacing */}
        <div className="p-4 rounded-xl bg-[#0D111A] border border-[#1E2638] space-y-2">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span className="flex items-center gap-1.5"><MessageSquare className="w-3.5 h-3.5 text-blue-400" /> Chat Rate</span>
            <span className="font-bold text-blue-400">{stats.chat_messages_per_min} msg/min</span>
          </div>
          <p className="text-[10px] text-slate-500">Natural jitter pacing</p>
        </div>
      </div>
    </div>
  );
};

export default AnalyticsPanel;
