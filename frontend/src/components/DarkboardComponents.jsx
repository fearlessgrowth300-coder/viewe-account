import React, { useState } from 'react';
import { 
  Play, Square, RefreshCw, Plus, Settings, Eye, MessageSquare, 
  Zap, CheckCircle2, AlertCircle, Clock, ShieldCheck, ChevronRight,
  TrendingUp, Radio, Users, Sliders, ExternalLink, X, ChevronDown, Check
} from 'lucide-react';

// ==============================================================================
// 1. BUTTON COMPONENTS KIT (PRIMARY, SECONDARY, SUCCESS, DANGER, OUTLINE, ICON)
// ==============================================================================
export const Button = ({ variant = 'primary', size = 'md', children, icon: Icon, className = '', ...props }) => {
  const base = "inline-flex items-center justify-center font-medium rounded-lg transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-[#0B0E14] active:scale-[0.98] cursor-pointer";
  
  const sizes = {
    sm: "px-3 py-1.5 text-xs gap-1.5",
    md: "px-4 py-2 text-sm gap-2",
    lg: "px-5 py-2.5 text-base gap-2.5",
    icon: "p-2 text-sm"
  };

  const variants = {
    primary: "bg-[#7C3AED] hover:bg-[#6D28D9] text-white shadow-lg shadow-[#7C3AED]/25 focus:ring-[#7C3AED]",
    secondary: "bg-[#1E2638] hover:bg-[#2B354C] text-slate-200 border border-slate-700/50 focus:ring-slate-500",
    success: "bg-[#10B981] hover:bg-[#059669] text-white shadow-lg shadow-[#10B981]/25 focus:ring-[#10B981]",
    danger: "bg-[#EF4444] hover:bg-[#DC2626] text-white shadow-lg shadow-[#EF4444]/25 focus:ring-[#EF4444]",
    outline: "bg-transparent border border-[#2B354C] text-slate-300 hover:border-[#7C3AED] hover:text-white focus:ring-[#7C3AED]"
  };

  return (
    <button className={`${base} ${sizes[size]} ${variants[variant]} ${className}`} {...props}>
      {Icon && <Icon className="w-4 h-4 shrink-0" />}
      {children}
    </button>
  );
};

// ==============================================================================
// 2. STATUS & PLATFORM BADGES
// ==============================================================================
export const StatusBadge = ({ status = 'running' }) => {
  const configs = {
    running: { bg: 'bg-emerald-500/10', text: 'text-emerald-400', dot: 'bg-emerald-400', label: 'Running' },
    active: { bg: 'bg-indigo-500/10', text: 'text-indigo-400', dot: 'bg-indigo-400', label: 'Active' },
    expired: { bg: 'bg-slate-500/10', text: 'text-slate-400', dot: 'bg-slate-400', label: 'Expired' },
    paused: { bg: 'bg-amber-500/10', text: 'text-amber-400', dot: 'bg-amber-400', label: 'Paused' },
    error: { bg: 'bg-rose-500/10', text: 'text-rose-400', dot: 'bg-rose-400', label: 'Error' },
    offline: { bg: 'bg-slate-600/10', text: 'text-slate-500', dot: 'bg-slate-500', label: 'Offline' }
  };

  const config = configs[status.toLowerCase()] || configs.running;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${config.bg} ${config.text} border border-current/20`}>
      <span className={`w-1.5 h-1.5 rounded-full ${config.dot} animate-pulse`} />
      {config.label}
    </span>
  );
};

export const PlatformBadge = ({ platform = 'twitch' }) => {
  const isTwitch = platform.toLowerCase() === 'twitch';
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold tracking-wider uppercase ${
      isTwitch ? 'bg-[#9146FF] text-white' : 'bg-[#53FC18] text-black font-black'
    }`}>
      {platform}
    </span>
  );
};

// ==============================================================================
// 3. STAT & METRIC CARDS
// ==============================================================================
export const MetricCard = ({ title, value, icon: Icon, color = "purple" }) => {
  const colors = {
    purple: "text-[#7C3AED] bg-[#7C3AED]/10 border-[#7C3AED]/20",
    green: "text-[#10B981] bg-[#10B981]/10 border-[#10B981]/20",
    blue: "text-[#3B82F6] bg-[#3B82F6]/10 border-[#3B82F6]/20"
  };

  return (
    <div className="bg-[#131826]/90 border border-[#1E2638] rounded-xl p-5 flex items-center justify-between backdrop-blur-md">
      <div>
        <p className="text-xs font-medium text-slate-400 uppercase tracking-wider">{title}</p>
        <p className="text-3xl font-extrabold text-white mt-1.5">{value}</p>
      </div>
      <div className={`p-3.5 rounded-xl border ${colors[color]}`}>
        <Icon className="w-6 h-6" />
      </div>
    </div>
  );
};

// ==============================================================================
// 4. TOGGLE SWITCH & SLIDERS
// ==============================================================================
export const ToggleSwitch = ({ enabled, onChange, label, description }) => {
  return (
    <div className="flex items-center justify-between py-2">
      <div>
        {label && <p className="text-sm font-medium text-slate-200">{label}</p>}
        {description && <p className="text-xs text-slate-400 mt-0.5">{description}</p>}
      </div>
      <button
        type="button"
        onClick={() => onChange(!enabled)}
        className={`${
          enabled ? 'bg-[#7C3AED]' : 'bg-[#1E2638]'
        } relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-[#7C3AED] focus:ring-offset-2 focus:ring-offset-[#0B0E14]`}
      >
        <span
          className={`${
            enabled ? 'translate-x-5 bg-white' : 'translate-x-0 bg-slate-400'
          } pointer-events-none inline-block h-5 w-5 transform rounded-full shadow-lg ring-0 transition duration-200 ease-in-out`}
        />
      </button>
    </div>
  );
};

// ==============================================================================
// 5. ADD CHANNEL MODAL (MATCHING UI FLOW 1)
// ==============================================================================
export const AddChannelModal = ({ isOpen, onClose, onAdd }) => {
  const [platform, setPlatform] = useState('twitch');
  const [channelName, setChannelName] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [disableViewerlist, setDisableViewerlist] = useState(false);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div className="w-full max-w-md bg-[#131826] border border-[#1E2638] rounded-2xl shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        <div className="flex items-center justify-between px-6 py-4 border-b border-[#1E2638]">
          <h2 className="text-lg font-bold text-white flex items-center gap-2">
            <Plus className="w-5 h-5 text-[#7C3AED]" /> Add Channel
          </h2>
          <button onClick={onClose} className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="p-6 space-y-5">
          {/* Select Platform */}
          <div>
            <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-2">Select Platform</label>
            <div className="grid grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => setPlatform('twitch')}
                className={`flex items-center justify-center gap-2.5 p-3 rounded-xl border font-semibold text-sm transition-all ${
                  platform === 'twitch' 
                    ? 'bg-[#9146FF]/15 border-[#9146FF] text-white shadow-lg shadow-[#9146FF]/10' 
                    : 'bg-[#0D111A] border-[#1E2638] text-slate-400 hover:border-slate-700'
                }`}
              >
                <PlatformBadge platform="twitch" /> Twitch
              </button>
              <button
                type="button"
                onClick={() => setPlatform('kick')}
                className={`flex items-center justify-center gap-2.5 p-3 rounded-xl border font-semibold text-sm transition-all ${
                  platform === 'kick' 
                    ? 'bg-[#53FC18]/10 border-[#53FC18] text-white shadow-lg shadow-[#53FC18]/10' 
                    : 'bg-[#0D111A] border-[#1E2638] text-slate-400 hover:border-slate-700'
                }`}
              >
                <PlatformBadge platform="kick" /> Kick
              </button>
            </div>
          </div>

          {/* Channel Name Input */}
          <div>
            <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-2">
              Enter your {platform} channel name
            </label>
            <div className="relative">
              <input
                type="text"
                placeholder="e.g. Zaybosays"
                value={channelName}
                onChange={(e) => setChannelName(e.target.value)}
                className="w-full bg-[#0D111A] border border-[#1E2638] rounded-xl px-4 py-2.5 text-white placeholder-slate-500 focus:outline-none focus:border-[#7C3AED] focus:ring-1 focus:ring-[#7C3AED]"
              />
              <span className="absolute right-3.5 top-3 text-xs text-slate-500">
                {platform}.tv/{channelName || 'username'}
              </span>
            </div>
          </div>

          {/* Advanced Options Accordion */}
          <div className="border border-[#1E2638] rounded-xl overflow-hidden">
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="w-full flex items-center justify-between px-4 py-3 bg-[#0D111A]/50 text-xs font-semibold text-slate-300 hover:text-white"
            >
              <span>Advanced options</span>
              <ChevronDown className={`w-4 h-4 transition-transform ${showAdvanced ? 'rotate-180' : ''}`} />
            </button>
            
            {showAdvanced && (
              <div className="p-4 bg-[#0D111A] space-y-4 border-t border-[#1E2638]">
                <ToggleSwitch
                  label="Disable Viewerlist"
                  description="Viewers won't appear in the channel viewer list."
                  enabled={disableViewerlist}
                  onChange={setDisableViewerlist}
                />
              </div>
            )}
          </div>

          {/* Submit */}
          <Button 
            variant="primary" 
            size="lg" 
            className="w-full mt-2" 
            onClick={() => {
              onAdd({ channelName, platform, disableViewerlist });
              onClose();
            }}
          >
            + Add Channel
          </Button>
        </div>
      </div>
    </div>
  );
};
