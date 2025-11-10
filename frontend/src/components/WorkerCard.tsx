import React from 'react';
import TaskCard from './TaskCard';

interface WorkerData {
  id: string;
  healthy: boolean;
  capacity?: number;
  current_tasks?: number;
  active_task_ids: string[];
  last_ping_seconds_ago?: number;
  cpu_usage_percent?: number;
  memory_usage_bytes?: number;
}

interface TaskData {
  id: string;
  type: string;
  status: string;
  job_id: string;
  worker_id: string;
  progress_percent?: number;
  records_in?: number;
  records_out?: number;
  attempt?: number;
  lease_expires_in_seconds?: number;
  byte_start?: number;
  byte_end?: number;
  input_file_count?: number;
}

interface WorkerCardProps {
  worker: WorkerData;
  activeTasks: TaskData[];
}

const WorkerCard: React.FC<WorkerCardProps> = ({ worker, activeTasks }) => {
  const capacity = worker.capacity ?? 0;
  const currentTasks = worker.current_tasks ?? 0;
  const cpuUsage = worker.cpu_usage_percent ?? 0;
  const memoryUsage = worker.memory_usage_bytes ?? 0;

  // Find tasks assigned to this worker
  const workerTasks = activeTasks.filter(task => 
    (worker.active_task_ids ?? []).includes(task.id)
  );

  const formatMemory = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
  };

  const getHealthColor = (healthy: boolean) => {
    return healthy ? 'text-green-600 bg-green-100' : 'text-red-600 bg-red-100';
  };
  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4 mb-4">
      <div className="flex items-start gap-4">
        {/* Worker Info Section */}
        <div className="shrink-0 w-80">
          {/* Header */}
          <div className="flex items-center justify-between mb-3">
            <div>
              <h3 className="text-sm font-semibold text-gray-900">
                Worker {worker.id.slice(-8)}
              </h3>
              <div className="flex items-center gap-2 mt-2">
                <span className={`text-xs font-medium px-2 py-1 rounded-full ${getHealthColor(worker.healthy)}`}>
                  {worker.healthy ? '● Healthy' : '● Unhealthy'}
                </span>
              </div>
            </div>
          </div>

          {/* Capacity */}
          <div className="mb-3">
            <div className="flex justify-between items-center mb-1">
              <span className="text-xs font-medium text-gray-600">Capacity</span>
              <span className="text-xs font-semibold text-gray-900">
                {currentTasks}/{capacity}
              </span>
            </div>
            <div className="w-full h-2 bg-gray-200 rounded-full overflow-hidden">
              <div 
                className="h-full bg-orange-500 rounded-full transition-all duration-300"
                style={{ width: `${capacity > 0 ? (currentTasks / capacity) * 100 : 0}%` }}
              />
            </div>
          </div>

          {/* Resource Usage */}
          <div className="grid grid-cols-2 gap-2">
            <div className="bg-gray-50 rounded p-2">
              <div className="text-xs text-gray-600 mb-1">CPU Usage</div>
              <div className="text-sm font-semibold text-gray-900">{cpuUsage.toFixed(1)}%</div>
              <div className="w-full h-1 bg-gray-200 rounded-full overflow-hidden mt-1">
                <div 
                  className="h-full bg-orange-400 rounded-full"
                  style={{ width: `${cpuUsage}%` }}
                />
              </div>
            </div>
            <div className="bg-gray-50 rounded p-2">
              <div className="text-xs text-gray-600 mb-1">Memory</div>
              <div className="text-sm font-semibold text-gray-900">{formatMemory(memoryUsage)}</div>
              <div className="text-xs text-gray-500 mt-1">
                {(worker.active_task_ids ?? []).length} active task{(worker.active_task_ids ?? []).length !== 1 ? 's' : ''}
              </div>
            </div>
          </div>
        </div>

        {/* Tasks Section */}
        <div className="flex-1">
          {workerTasks.length > 0 ? (
            <div>
              <div className="text-xs font-medium text-gray-600 mb-2">
                Active Tasks ({workerTasks.length})
              </div>
              <div className="overflow-x-auto">
                <div className="flex gap-3">
                  {workerTasks.map(task => (
                    <TaskCard key={task.id} task={task} />
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-center h-full">
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default WorkerCard;