import React from 'react';
import SystemOverview from './SystemOverview';
import JobsOverview from './JobsOverview';
import WorkersOverview from './WorkersOverview';
import type { DashboardState, FileSystemNode } from '../hooks/useWebSocket';

interface DashboardProps {
  data: DashboardState | null;
  volumeDirectory: FileSystemNode[] | null;
  connectionStatus: 'connecting' | 'connected' | 'disconnected';
}

const Dashboard: React.FC<DashboardProps> = ({ data, connectionStatus }) => {
  return (
    <div className="flex h-full">
      {/* Dashboard Content */}
      <div className="flex-1 p-8 overflow-y-auto mt-18">
        <div className="max-w-6xl mx-auto">
          <div className='flex flex-row justify-between items-center mb-4'>
            <h1 className="text-4xl font-bold text-gray-900 ">Dashboard</h1>

            {/* Connection Status */}
            <div className="">
              <div className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
                connectionStatus === 'connected' 
                  ? 'bg-green-100 text-green-800' 
                  : connectionStatus === 'connecting'
                  ? 'bg-yellow-100 text-yellow-800'
                  : 'bg-red-100 text-red-800'
              }`}>
                <div className={`w-2 h-2 rounded-full mr-2 ${
                  connectionStatus === 'connected' 
                    ? 'bg-green-400' 
                    : connectionStatus === 'connecting'
                    ? 'bg-yellow-400'
                    : 'bg-red-400'
                }`} />
                WebSocket: {connectionStatus}
              </div>
            </div>
          </div>
          
          

          {/* System Overview */}
          <SystemOverview data={data} />

          {/* Jobs Overview */}
          <JobsOverview data={data} />

          {/* Workers Overview */}
          <WorkersOverview data={data} />
        </div>
      </div>
    </div>
  );
};

export default Dashboard;