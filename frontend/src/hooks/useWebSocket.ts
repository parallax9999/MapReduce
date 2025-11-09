import { useEffect, useRef, useState } from 'react';

export interface DashboardState {
  system: {
    total_workers: number;
    healthy_workers: number;
    total_capacity: number;
    used_capacity: number;
    pending_task_count: number;
    total_jobs: number;
    active_jobs: number;
  };
  workers: Array<{
    id: string;
    healthy: boolean;
    capacity: number;
    current_tasks: number;
    active_task_ids: string[];
    last_ping_seconds_ago: number;
    cpu_usage_percent: number;
    memory_usage_bytes: number;
  }>;
  jobs: Array<{
    id: string;
    phase: string;
    map_tasks_done: number;
    map_tasks_total: number;
    reduce_tasks_done: number;
    reduce_tasks_total: number;
    overall_progress: number;
    mapper_count: number;
    reducer_count: number;
    created_seconds_ago: number;
    enable_combiner: boolean;
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
    progress_percent: number;
    records_in: number;
    records_out: number;
    attempt: number;
    lease_expires_in_seconds: number;
    byte_start: number;
    byte_end: number;
    input_file_count: number;
  }>;
  pending_tasks: Array<{
    id: string;
    type: string;
    status: string;
    job_id: string;
    worker_id: string;
    progress_percent: number;
    records_in: number;
    records_out: number;
    attempt: number;
    lease_expires_in_seconds: number;
    byte_start: number;
    byte_end: number;
    input_file_count: number;
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
                setData(message.data as DashboardState);
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