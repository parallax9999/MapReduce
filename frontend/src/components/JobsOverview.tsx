import React from 'react';
import JobCard from './JobCard';
import type { DashboardState } from '../hooks/useWebSocket';

interface JobsOverviewProps {
  data: DashboardState | null;
}

const JobsOverview: React.FC<JobsOverviewProps> = ({ data }) => {
  if (!data || !data.jobs || data.jobs.length === 0) {
    return (
      <div className="mt-8">
        <h2 className="text-xl font-semibold text-gray-900 mb-4">Active Jobs</h2>
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8">
          <div className="text-center text-gray-500">
            No active jobs at the moment
          </div>
        </div>
      </div>
    );
  }

  // Sort jobs by created_seconds_ago (most recent first)
  const sortedJobs = [...data.jobs].sort((a, b) => (a.created_seconds_ago || 0) - (b.created_seconds_ago || 0));

  return (
    <div className="mt-8">
      
      <div className="overflow-x-auto pb-4">
        <div className="flex gap-4">
          {sortedJobs.map((job) => (
            <JobCard key={job.id} job={job} />
          ))}
        </div>
      </div>
    </div>
  );
};

export default JobsOverview;