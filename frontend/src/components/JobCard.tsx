import React from 'react';

interface JobData {
  id: string;
  phase: string;
  map_tasks_done?: number;
  map_tasks_total?: number;
  reduce_tasks_done?: number;
  reduce_tasks_total?: number;
  overall_progress?: number;
  mapper_count?: number;
  reducer_count?: number;
  created_seconds_ago?: number;
  enable_combiner?: boolean;
  input_files: string[];
  output_path: string;
  code_uri: string;
}

interface JobCardProps {
  job: JobData;
}

const JobCard: React.FC<JobCardProps> = ({ job }) => {
  const mapTasksDone = job.map_tasks_done ?? 0;
  const mapTasksTotal = job.map_tasks_total ?? 0;
  const reduceTasksDone = job.reduce_tasks_done ?? 0;
  const reduceTasksTotal = job.reduce_tasks_total ?? 0;
  const overallProgress = job.overall_progress ?? 0;
  const mapperCount = job.mapper_count ?? 0;
  const reducerCount = job.reducer_count ?? 0;
  const createdSecondsAgo = job.created_seconds_ago ?? 0;
  const enableCombiner = job.enable_combiner ?? false;
  
  const mapProgress = mapTasksTotal > 0 ? (mapTasksDone / mapTasksTotal) * 100 : 0;
  const reduceProgress = reduceTasksTotal > 0 ? (reduceTasksDone / reduceTasksTotal) * 100 : 0;
  const isDone = job.phase.toLowerCase() === 'completed' || job.phase.toLowerCase() === 'done';
  
  const formatTime = (seconds: number) => {
    if (seconds < 60) return `${seconds}s ago`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
    return `${Math.floor(seconds / 3600)}h ago`;
  };

  return (
    <div className={`rounded-lg shadow-sm border p-4 shrink-0 w-80 ${
      isDone ? 'bg-gray-50 border-gray-100 opacity-60' : 'bg-white border-gray-200'
    }`}>
      {/* Header */}
      <div className="flex justify-between items-start mb-3">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 truncate">Job {job.id.slice(31, 50)}</h3>
          <p className={`text-xs font-medium text-orange-500 mt-1`}>
            {job.phase.toUpperCase()}
          </p>
        </div>
        <span className="text-xs text-gray-500">{formatTime(createdSecondsAgo)}</span>
      </div>

      {/* Overall Progress */}
      <div className="mb-3">
        <div className="flex justify-between items-center mb-1">
          <span className="text-xs font-medium text-gray-600">Overall Progress</span>
          <span className="text-xs font-semibold text-gray-900">{Math.round(overallProgress)}%</span>
        </div>
        <div className="w-full h-2 bg-gray-200 rounded-full overflow-hidden">
          <div 
            className="h-full bg-orange-500 transition-all duration-300 ease-out rounded-full"
            style={{ width: `${overallProgress}%` }}
          />
        </div>
      </div>

      {/* Map/Reduce Tasks */}
      <div className="grid grid-cols-2 gap-2 mb-3">
        <div className="bg-gray-50 rounded p-2">
          <div className="text-xs text-gray-600 mb-1">Map Tasks</div>
          <div className="text-sm font-semibold text-gray-900">
            {mapTasksDone}/{mapTasksTotal}
          </div>
          <div className="w-full h-1 bg-gray-200 rounded-full overflow-hidden mt-1">
            <div 
              className="h-full bg-orange-500 rounded-full"
              style={{ width: `${mapProgress}%` }}
            />
          </div>
        </div>
        <div className="bg-gray-50 rounded p-2">
          <div className="text-xs text-gray-600 mb-1">Reduce Tasks</div>
          <div className="text-sm font-semibold text-gray-900">
            {reduceTasksDone}/{reduceTasksTotal}
          </div>
          <div className="w-full h-1 bg-gray-200 rounded-full overflow-hidden mt-1">
            <div 
              className="h-full bg-orange-500 rounded-full"
              style={{ width: `${reduceProgress}%` }}
            />
          </div>
        </div>
      </div>

      {/* Worker Configuration */}
      <div className="grid grid-cols-3 gap-2 mb-3 text-xs">
        <div className="text-center">
          <div className="text-gray-600 mb-1">Mappers</div>
          <div className="font-semibold text-gray-900">{mapperCount}</div>
        </div>
        <div className="text-center">
          <div className="text-gray-600 mb-1">Reducers</div>
          <div className="font-semibold text-gray-900">{reducerCount}</div>
        </div>
        <div className="text-center">
          <div className="text-gray-600 mb-1">Combiner</div>
          <div className="font-semibold">
            {enableCombiner ? (
              <span className="text-green-600">✓</span>
            ) : (
              <span className="text-red-600">✗</span>
            )}
          </div>
        </div>
      </div>

      {/* Code URI */}
      <div className="mb-2">
        <div className="text-xs text-gray-600 mb-1">Code</div>
        <div className="text-xs font-mono bg-gray-100 px-2 py-1 rounded truncate text-gray-800">
          {job.code_uri}
        </div>
      </div>

      {/* Input Files */}
      <div>
        <div className="text-xs text-gray-600 mb-1">Input Files ({job.input_files.length})</div>
        <div className="text-xs space-y-1">
          {job.input_files.slice(0, 2).map((file, idx) => (
            <div key={idx} className="font-mono bg-gray-100 px-2 py-1 rounded truncate text-gray-800">
              {file}
            </div>
          ))}
          {job.input_files.length > 2 && (
            <div className="text-gray-500 italic">+{job.input_files.length - 2} more...</div>
          )}
        </div>
      </div>
    </div>
  );
};

export default JobCard;