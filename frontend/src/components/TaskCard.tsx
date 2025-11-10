import React from 'react';

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

interface TaskCardProps {
  task: TaskData;
}

const TaskCard: React.FC<TaskCardProps> = ({ task }) => {
  const progressPercent = task.progress_percent ?? 0;
  const recordsIn = task.records_in ?? 0;
  const recordsOut = task.records_out ?? 0;
  const attempt = task.attempt ?? 0;
  const leaseExpiresIn = task.lease_expires_in_seconds ?? 0;
  const inputFileCount = task.input_file_count ?? 0;

  const getTypeIcon = (type: string) => {
    if (type.toLowerCase().includes('map')) {
      return (
        <svg className="w-4 h-4 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
        </svg>
      );
    } else if (type.toLowerCase().includes('reduce')) {
      return (
        <svg className="w-4 h-4 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
      );
    }
    return (
      <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
      </svg>
    );
  };

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-3 shrink-0 w-64">
      {/* Header */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          {getTypeIcon(task.type)}
          <div>
            <div className="text-xs font-semibold text-gray-900 truncate">
              {task.type.toUpperCase()}
            </div>
            <div className="text-xs text-gray-500">
              Task {task.id.slice(-8)}
            </div>
          </div>
        </div>
        <span className={`text-xs font-medium px-2 py-1 rounded-full text-gray-500`}>
          {task.status}
        </span>
      </div>

      {/* Progress */}
      <div className="mb-2">
        <div className="flex justify-between items-center mb-1">
          <span className="text-xs text-gray-600">Progress</span>
          <span className="text-xs font-semibold text-gray-900">{Math.round(progressPercent)}%</span>
        </div>
        <div className="w-full h-1.5 bg-gray-200 rounded-full overflow-hidden">
          <div 
            className="h-full bg-orange-500 rounded-full transition-all duration-300"
            style={{ width: `${progressPercent}%` }}
          />
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-2 text-xs">
        <div className="bg-gray-50 rounded px-2 py-1">
          <div className="text-gray-600">Records In</div>
          <div className="font-semibold text-gray-900">{recordsIn.toLocaleString()}</div>
        </div>
        <div className="bg-gray-50 rounded px-2 py-1">
          <div className="text-gray-600">Records Out</div>
          <div className="font-semibold text-gray-900">{recordsOut.toLocaleString()}</div>
        </div>
      </div>

      {/* Additional Info */}
      <div className="mt-2 flex justify-between text-xs text-gray-500">
        <span>Attempt {attempt}</span>
        {inputFileCount > 0 && (
          <span>{inputFileCount} file{inputFileCount !== 1 ? 's' : ''}</span>
        )}
      </div>
    </div>
  );
};

export default TaskCard;