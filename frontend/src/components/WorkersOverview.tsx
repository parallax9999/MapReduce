import React from 'react';
import WorkerCard from './WorkerCard';
import type { DashboardState } from '../hooks/useWebSocket';

interface WorkersOverviewProps {
  data: DashboardState | null;
}

const WorkersOverview: React.FC<WorkersOverviewProps> = ({ data }) => {
  if (!data || !data.workers || data.workers.length === 0) {
    return (
      <div className="mt-4">
        <h2 className="text-xl font-semibold text-gray-900 mb-4">Workers</h2>
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8">
          <div className="text-center text-gray-500">
            No workers connected
          </div>
        </div>
      </div>
    );
  }

  // Sort workers by id for consistent ordering
  const sortedWorkers = [...data.workers].sort((a, b) => a.id.localeCompare(b.id));

  return (
    <div className="mt-4">      
      <div className="space-y-4">
        {sortedWorkers.map(worker => (
          <WorkerCard 
            key={worker.id} 
            worker={worker} 
            activeTasks={data.active_tasks || []}
          />
        ))}
      </div>
    </div>
  );
};

export default WorkersOverview;