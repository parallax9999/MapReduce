import React from 'react';
import type { DashboardState } from '../hooks/useWebSocket';

interface SystemOverviewProps {
  data: DashboardState | null;
}

interface ProgressPillProps {
  current: number;
  total: number;
  label: string;
  color?: string;
}

const ProgressPill: React.FC<ProgressPillProps> = ({ current, total, label }) => {
  const percentage = total > 0 ? (current / total) * 100 : 0;
  
  return (
    <div className="bg-white rounded-lg shadow-sm p-4">
      <div className="mb-2 flex justify-between items-center">
        <span className="text-sm font-medium text-gray-700">{label}</span>
        <span className="text-lg font-bold text-gray-900">
          {current}/{total}
        </span>
      </div>
      <div className="w-full h-6 bg-gray-200 rounded-full overflow-hidden">
        <div 
          className={`h-full bg-orange-500 transition-all duration-300 ease-out rounded-full`}
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  );
};

interface StatCardProps {
  label: string;
  value: number | string;
  icon?: React.ReactNode;
  color?: string;
}

const StatCard: React.FC<StatCardProps> = ({ label, value, icon, color = "text-blue-600" }) => {
  return (
    <div className="bg-white rounded-lg shadow-sm p-4">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-gray-600 mb-1">{label}</p>
          <p className={`text-2xl font-bold ${color}`}>{value}</p>
        </div>
        {icon && (
          <div className="ml-4">
            {icon}
          </div>
        )}
      </div>
    </div>
  );
};

const SystemOverview: React.FC<SystemOverviewProps> = ({ data }) => {
  if (!data) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map(i => (
          <div key={i} className="bg-white rounded-lg shadow-sm p-4 animate-pulse">
            <div className="h-4 bg-gray-200 rounded w-3/4 mb-2"></div>
            <div className="h-8 bg-gray-200 rounded w-1/2"></div>
          </div>
        ))}
      </div>
    );
  }

  const totalTasks = data.active_tasks.length + data.pending_tasks.length;
  const completedJobs = (data.system.total_jobs || 0) - (data.system.active_jobs || 0);

  return (
    <div className="space-y-6">
      <div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <ProgressPill
            current={data.system.healthy_workers || 0}
            total={data.system.total_workers || 0}
            label="Healthy Workers"
          />
          
          <ProgressPill
            current={data.pending_tasks.length}
            total={totalTasks}
            label="Pending Tasks / Total Tasks"
          />
          
          <StatCard
            label="Active Jobs"
            value={data.system.active_jobs || 0}
            color="text-orange-500"
            icon={
              <svg className="w-8 h-8 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            }
          />
          
          <StatCard
            label="Completed Jobs"
            value={completedJobs}
            color="text-orange-500"
            icon={
              <svg className="w-8 h-8 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            }
          />
        </div>
      </div>
    </div>
  );
};

export default SystemOverview;