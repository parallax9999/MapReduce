import { useEffect, useRef, useState } from 'react';

export interface DashboardState {
  system: {
    total_workers?: number;
    healthy_workers?: number;
    total_capacity?: number;
    used_capacity?: number;
    pending_task_count?: number;
    total_jobs?: number;
    active_jobs?: number;
  };
  workers: Array<{
    id: string;
    healthy: boolean;
    capacity?: number;
    current_tasks?: number;
    active_task_ids: string[];
    last_ping_seconds_ago?: number;
    cpu_usage_percent?: number;
    memory_usage_bytes?: number;
  }>;
  jobs: Array<{
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
  }>;
  active_tasks: Array<{
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
  }>;
  pending_tasks: Array<{
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
  }>;
  timestamp: number;
}

export interface FileSystemNode {
  name: string;
  type: 'file' | 'directory';
  path: string;
  children?: FileSystemNode[];
}

export interface WebSocketMessage {
  type: string;
  data: any;
}

// Normalize dashboard state by filling in zero values for missing numeric fields
const normalizeDashboardState = (state: DashboardState): DashboardState => {
  return {
    ...state,
    system: {
      total_workers: state.system?.total_workers ?? 0,
      healthy_workers: state.system?.healthy_workers ?? 0,
      total_capacity: state.system?.total_capacity ?? 0,
      used_capacity: state.system?.used_capacity ?? 0,
      pending_task_count: state.system?.pending_task_count ?? 0,
      total_jobs: state.system?.total_jobs ?? 0,
      active_jobs: state.system?.active_jobs ?? 0,
    },
    workers: state.workers?.map(worker => ({
      ...worker,
      capacity: worker.capacity ?? 0,
      current_tasks: worker.current_tasks ?? 0,
      last_ping_seconds_ago: worker.last_ping_seconds_ago ?? 0,
      cpu_usage_percent: worker.cpu_usage_percent ?? 0,
      memory_usage_bytes: worker.memory_usage_bytes ?? 0,
    })) ?? [],
    jobs: state.jobs?.map(job => ({
      ...job,
      map_tasks_done: job.map_tasks_done ?? 0,
      map_tasks_total: job.map_tasks_total ?? 0,
      reduce_tasks_done: job.reduce_tasks_done ?? 0,
      reduce_tasks_total: job.reduce_tasks_total ?? 0,
      overall_progress: job.overall_progress ?? 0,
      mapper_count: job.mapper_count ?? 0,
      reducer_count: job.reducer_count ?? 0,
      created_seconds_ago: job.created_seconds_ago ?? 0,
      enable_combiner: job.enable_combiner ?? false,
    })) ?? [],
    active_tasks: state.active_tasks?.map(task => ({
      ...task,
      type: task.type ?? 'map',
      progress_percent: task.progress_percent ?? 0,
      records_in: task.records_in ?? 0,
      records_out: task.records_out ?? 0,
      attempt: task.attempt ?? 0,
      lease_expires_in_seconds: task.lease_expires_in_seconds ?? 0,
      byte_start: task.byte_start ?? 0,
      byte_end: task.byte_end ?? 0,
      input_file_count: task.input_file_count ?? 0,
    })) ?? [],
    pending_tasks: state.pending_tasks?.map(task => ({
      ...task,
      type: task.type ?? 'map',
      progress_percent: task.progress_percent ?? 0,
      records_in: task.records_in ?? 0,
      records_out: task.records_out ?? 0,
      attempt: task.attempt ?? 0,
      lease_expires_in_seconds: task.lease_expires_in_seconds ?? 0,
      byte_start: task.byte_start ?? 0,
      byte_end: task.byte_end ?? 0,
      input_file_count: task.input_file_count ?? 0,
    })) ?? [],
    timestamp: state.timestamp ?? Date.now() / 1000,
  };
};

export const useWebSocket = (url: string) => {
  const [data, setData] = useState<DashboardState | null>(null);
  const [volumeDirectory, setVolumeDirectory] = useState<FileSystemNode[] | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'disconnected'>('disconnected');
  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    const connectWebSocket = () => {
      try {
        setConnectionStatus('connecting');
        ws.current = new WebSocket(url);

        ws.current.onopen = () => {
          console.log('WebSocket connected to:', url);
          setConnectionStatus('connected');
        };

        ws.current.onmessage = (event) => {
          try {
            const message: WebSocketMessage = JSON.parse(event.data);
            console.log('Received WebSocket message:', message.type);
            
            switch (message.type) {
              case 'dashboard_state':
                // Normalize the dashboard state by filling in zero values
                const normalizedData = normalizeDashboardState(message.data as DashboardState);
                setData(normalizedData);
                break;
              case 'volume_directory':
                setVolumeDirectory(message.data as FileSystemNode[]);
                break;
              default:
                console.warn('Unknown WebSocket message type:', message.type);
            }
          } catch (error) {
            console.error('Failed to parse WebSocket data:', error);
          }
        };

        ws.current.onclose = () => {
          console.log('WebSocket disconnected');
          setConnectionStatus('disconnected');
          
          // Attempt to reconnect after 3 seconds
          setTimeout(() => {
            if (ws.current?.readyState === WebSocket.CLOSED) {
              connectWebSocket();
            }
          }, 3000);
        };

        ws.current.onerror = (error) => {
          console.error('WebSocket error:', error);
          setConnectionStatus('disconnected');
        };

      } catch (error) {
        console.error('Failed to create WebSocket connection:', error);
        setConnectionStatus('disconnected');
      }
    };

    connectWebSocket();

    // Cleanup on component unmount
    return () => {
      if (ws.current) {
        ws.current.close();
      }
    };
  }, [url]);

  return { data, volumeDirectory, connectionStatus };
};