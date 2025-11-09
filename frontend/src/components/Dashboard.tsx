import React from 'react';
import FileBrowser from './FileBrowser';
import type { DashboardState, FileSystemNode } from '../hooks/useWebSocket';

interface DashboardProps {
  data: DashboardState | null;
  volumeDirectory: FileSystemNode[] | null;
  connectionStatus: 'connecting' | 'connected' | 'disconnected';
}

const Dashboard: React.FC<DashboardProps> = ({ data, volumeDirectory, connectionStatus }) => {
  return (
    <div className="flex h-full">
      {/* Dashboard Content */}
      <div className="flex-1 p-8 overflow-y-auto">
        <div className="max-w-6xl mx-auto">
          <h1 className="text-4xl font-bold text-gray-900 mb-8">MapReduce Dashboard</h1>
          
          {/* Connection Status */}
          <div className="mb-6">
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

          {/* Dashboard Data Display */}
          {data ? (
            <div className="bg-white rounded-lg shadow p-6">
              <h2 className="text-2xl font-semibold mb-4">Live Dashboard Data</h2>
              <pre className="bg-gray-50 p-4 rounded-lg text-sm overflow-auto max-h-96">
                {JSON.stringify(data, null, 2)}
              </pre>
              <div className="mt-4 text-sm text-gray-500">
                Last updated: {new Date(data.timestamp * 1000).toLocaleTimeString()}
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-lg shadow p-6">
              <div className="text-center text-gray-500">
                {connectionStatus === 'connected' 
                  ? 'Waiting for dashboard data...' 
                  : 'Not connected to MapReduce system'}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Dashboard;