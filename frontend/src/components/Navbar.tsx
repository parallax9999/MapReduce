import React from 'react';

interface NavbarProps {
  activeTab: 'dashboard' | 'start-job';
  onTabChange: (tab: 'dashboard' | 'start-job') => void;
}

const Navbar: React.FC<NavbarProps> = ({ activeTab, onTabChange }) => {
  return (
    <div className="absolute w-full bg-white border-b border-gray-200 shadow-sm">
      <div className="px-6">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center">
            <h1 className="text-xl font-bold text-gray-900">MapReduce System</h1>
          </div>
          
          <nav className="flex space-x-8">
            <button
              onClick={() => onTabChange('dashboard')}
              className={`py-2 px-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'dashboard'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              Dashboard
            </button>
            <button
              onClick={() => onTabChange('start-job')}
              className={`py-2 px-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'start-job'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              Start Job
            </button>
          </nav>
        </div>
      </div>
    </div>
  );
};

export default Navbar;