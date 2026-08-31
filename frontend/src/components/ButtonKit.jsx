import React from 'react';
import { 
  Plus, 
  RefreshCw, 
  Play, 
  Square, 
  Settings, 
  MoreVertical, 
  Tv, 
  Radio, 
  Loader2 
} from 'lucide-react';

/**
 * ============================================================================
 * 1. REUSABLE BUTTON COMPONENT
 * ============================================================================
 * Supports:
 * - Variants: 'primary' | 'secondary' | 'success' | 'danger' | 'outline' | 'icon'
 * - Sizes: 'sm' | 'md' | 'lg'
 * - Loading & Disabled states
 * - Custom Left/Right Icons
 */
export const Button = ({
  variant = 'primary',
  size = 'md',
  children,
  icon: Icon,
  rightIcon: RightIcon,
  isLoading = false,
  disabled = false,
  className = '',
  ...props
}) => {
  // Base styling: smooth transitions, active press feedback, focus rings
  const baseStyles = 
    "inline-flex items-center justify-center font-medium rounded-xl transition-all duration-200 " +
    "focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-[#0B0E14] " +
    "active:scale-[0.98] disabled:opacity-50 disabled:pointer-events-none disabled:cursor-not-allowed cursor-pointer";

  // Sizing definitions matching the reference mockup
  const sizes = {
    sm: "px-3 py-1.5 text-xs gap-1.5",
    md: "px-4 py-2 text-sm gap-2",
    lg: "px-5 py-2.5 text-base gap-2.5",
    icon: "p-2.5 text-sm"
  };

  // State-specific color variants matching the exact reference UI design
  const variants = {
    // Primary (+ Add Channel): Deep neon purple/violet with subtle glow
    primary: 
      "bg-[#7C3AED] hover:bg-[#6D28D9] text-white shadow-lg shadow-[#7C3AED]/25 " +
      "border border-[#8B5CF6]/40 focus:ring-[#7C3AED]",

    // Secondary (Update): Dark slate with subtle border
    secondary: 
      "bg-[#161B26] hover:bg-[#1E2638] text-slate-200 border border-[#2B354C] " +
      "hover:border-slate-500 focus:ring-slate-500",

    // Success (Start): Emerald green with action glow
    success: 
      "bg-[#10B981] hover:bg-[#059669] text-white shadow-lg shadow-[#10B981]/25 " +
      "border border-[#34D399]/40 focus:ring-[#10B981]",

    // Danger (Stop): Coral red with stop glow
    danger: 
      "bg-[#EF4444] hover:bg-[#DC2626] text-white shadow-lg shadow-[#EF4444]/25 " +
      "border border-[#F87171]/40 focus:ring-[#EF4444]",

    // Outline (Manage): Transparent card background with light gray border
    outline: 
      "bg-transparent border border-[#2B354C] text-slate-300 " +
      "hover:border-[#7C3AED] hover:text-white hover:bg-[#161B26]/50 focus:ring-[#7C3AED]",

    // Icon only buttons
    icon: 
      "bg-[#161B26] hover:bg-[#1E2638] text-slate-300 border border-[#2B354C] " +
      "hover:text-white hover:border-[#6366F1] focus:ring-[#6366F1]"
  };

  return (
    <button
      disabled={disabled || isLoading}
      className={`${baseStyles} ${sizes[size]} ${variants[variant]} ${className}`}
      {...props}
    >
      {isLoading ? (
        <Loader2 className="w-4 h-4 animate-spin shrink-0" />
      ) : (
        Icon && <Icon className="w-4 h-4 shrink-0" />
      )}
      {children}
      {RightIcon && !isLoading && <RightIcon className="w-4 h-4 shrink-0" />}
    </button>
  );
};

/**
 * ============================================================================
 * 2. ICON BUTTON COMPONENT
 * ============================================================================
 */
export const IconButton = ({ icon: Icon, className = '', ...props }) => (
  <Button variant="icon" size="icon" className={`rounded-xl ${className}`} {...props}>
    {Icon && <Icon className="w-4 h-4" />}
  </Button>
);

/**
 * ============================================================================
 * 3. "ALL BUTTONS" CARD (Exact Match to the Mockup Reference)
 * ============================================================================
 */
export const AllButtonsCard = () => {
  return (
    <div className="bg-[#161B26] border border-[#1E2638] rounded-2xl p-6 space-y-6 max-w-4xl text-slate-200">
      <div className="border-b border-[#1E2638] pb-3">
        <h2 className="text-base font-bold text-white tracking-wide">All Buttons</h2>
        <p className="text-xs text-slate-400 mt-0.5">Component states and variants from the design system</p>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4 items-center">
        {/* 1. Primary Button */}
        <div className="space-y-2">
          <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Primary</span>
          <Button variant="primary" icon={Plus}>
            Add Channel
          </Button>
        </div>

        {/* 2. Secondary Button */}
        <div className="space-y-2">
          <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Secondary</span>
          <Button variant="secondary" icon={RefreshCw}>
            Update
          </Button>
        </div>

        {/* 3. Success Button */}
        <div className="space-y-2">
          <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Success</span>
          <Button variant="success" icon={Play}>
            Start
          </Button>
        </div>

        {/* 4. Danger Button */}
        <div className="space-y-2">
          <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Danger</span>
          <Button variant="danger" icon={Square}>
            Stop
          </Button>
        </div>

        {/* 5. Outline Button */}
        <div className="space-y-2">
          <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Outline</span>
          <Button variant="outline" icon={Settings}>
            Manage
          </Button>
        </div>

        {/* 6. Icon Buttons Group */}
        <div className="space-y-2">
          <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Icon Buttons</span>
          <div className="flex items-center gap-2">
            <IconButton icon={Tv} title="Twitch" />
            <IconButton icon={Radio} title="Kick" />
            <IconButton icon={MoreVertical} title="More Options" />
          </div>
        </div>
      </div>
    </div>
  );
};

export default Button;
