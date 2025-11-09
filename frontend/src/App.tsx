import { useState } from 'react'
import { useWebSocket } from './hooks/useWebSocket'
import Navbar from './components/Navbar'
import Dashboard from './components/Dashboard'
import StartJob from './components/StartJob'

function App() {
  const { data, volumeDirectory, connectionStatus } = useWebSocket('ws://localhost:8081/ws');
  const [activeTab, setActiveTab] = useState<'dashboard' | 'start-job'>('dashboard');

  return (
    <div className="min-h-screen bg-gray-100 flex flex-col">
      {/* Navbar */}
      <Navbar activeTab={activeTab} onTabChange={setActiveTab} />
      
      {/* Main Content Area */}
      <div>
        {activeTab === 'dashboard' ? (
          <Dashboard 
            data={data} 
            volumeDirectory={volumeDirectory} 
            connectionStatus={connectionStatus} 
          />
        ) : (
          <StartJob volumeDirectory={volumeDirectory} />
        )}
      </div>
    </div>
  )
}

export default App
