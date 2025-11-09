import React, { useState } from 'react';
import FileBrowser from './FileBrowser';
import type { FileSystemNode } from '../hooks/useWebSocket';

interface StartJobProps {
  volumeDirectory: FileSystemNode[] | null;
}

const StartJob: React.FC<StartJobProps> = ({ volumeDirectory }) => {
  const [formData, setFormData] = useState({
    codeUri: '/wordcount.py',
    inputFiles: '/input.csv',
    mapperCount: 2,
    reducerCount: 2,
    enableCombiner: true,
    inputType: 'TEXT',
    outputType: 'TEXT',
    outputDir: '/output'
  });

  const [uploadData, setUploadData] = useState({
    file: null as File | null,
    path: ''
  });

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value, type } = e.target;
    
    if (type === 'checkbox') {
      const checked = (e.target as HTMLInputElement).checked;
      setFormData(prev => ({ ...prev, [name]: checked }));
    } else if (type === 'number') {
      setFormData(prev => ({ ...prev, [name]: parseInt(value) || 0 }));
    } else {
      setFormData(prev => ({ ...prev, [name]: value }));
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setUploadData(prev => ({ ...prev, file }));
    }
  };

  const handleUploadPathChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setUploadData(prev => ({ ...prev, path: e.target.value }));
  };

  const handleFileUpload = async () => {
    if (!uploadData.file || !uploadData.path) {
      alert('Please select a file and enter an upload path');
      return;
    }

    try {
      const formData = new FormData();
      formData.append('file', uploadData.file);
      formData.append('path', uploadData.path);

      const response = await fetch('http://localhost:8081/api/upload-file', {
        method: 'POST',
        body: formData,
      });

      const result = await response.json();

      if (result.success) {
        alert(`File uploaded successfully to ${result.path}`);
        // Reset upload form
        setUploadData({ file: null, path: '' });
        // Clear file input
        const fileInput = document.getElementById('fileInput') as HTMLInputElement;
        if (fileInput) fileInput.value = '';
      } else {
        alert(`Failed to upload file: ${result.message}`);
      }
    } catch (error) {
      console.error('Error uploading file:', error);
      alert('Failed to upload file. Please check your connection.');
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    console.log('Submitting job with data:', formData);
    
    try {
      // Parse input files from comma-separated string
      const inputFiles = formData.inputFiles.split(',').map(file => file.trim()).filter(file => file.length > 0);
      
      const response = await fetch('http://localhost:8081/api/submit-job', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          codeUri: formData.codeUri,
          inputFiles: inputFiles,
          mapperCount: formData.mapperCount,
          reducerCount: formData.reducerCount,
          enableCombiner: formData.enableCombiner,
          inputType: formData.inputType,
          outputType: formData.outputType,
          outputDir: formData.outputDir,
        }),
      });
      
      const result = await response.json();
      
      if (result.success) {
        alert(`Job submitted successfully! Job ID: ${result.jobId}`);
        // Reset form or show success message
      } else {
        alert(`Failed to submit job: ${result.message}`);
      }
    } catch (error) {
      console.error('Error submitting job:', error);
      alert('Failed to submit job. Please check your connection.');
    }
  };

  return (
    <div className="flex flex-row h-screen">
      {/* File Browser Sidebar */}
      <div className="w-56 h-full">
        <FileBrowser directory={volumeDirectory} />
      </div>
      <div className='flex w-full h-full pt-18'>
        <div className='flex flex-col justify-center gap-2 md:flex-row w-full h-full overflow-auto'>

          {/* Submit form */}
          <div className="max-w-3xl mx-5 mt-4">
            <h2 className="text-2xl font-bold text-gray-900 mb-6">Submit MapReduce Job</h2>
            
            <form onSubmit={handleSubmit} className="space-y-6">
              <div className="bg-white shadow rounded-lg p-6">
                <h3 className="text-lg font-medium text-gray-900 mb-4">Job Configuration</h3>
                
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Code File Path
                    </label>
                    <input
                      type="text"
                      name="codeUri"
                      value={formData.codeUri}
                      onChange={handleInputChange}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="/path/to/your/mapper_reducer.py"
                      required
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Input Files (comma-separated)
                    </label>
                    <input
                      type="text"
                      name="inputFiles"
                      value={formData.inputFiles}
                      onChange={handleInputChange}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="/input1.csv, /input2.csv"
                      required
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">
                        Number of Mappers
                      </label>
                      <input
                        type="number"
                        name="mapperCount"
                        value={formData.mapperCount}
                        onChange={handleInputChange}
                        min="1"
                        max="10"
                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        required
                      />
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">
                        Number of Reducers
                      </label>
                      <input
                        type="number"
                        name="reducerCount"
                        value={formData.reducerCount}
                        onChange={handleInputChange}
                        min="1"
                        max="10"
                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        required
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">
                        Input Format
                      </label>
                      <select
                        name="inputType"
                        value={formData.inputType}
                        onChange={handleInputChange}
                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                      >
                        <option value="TEXT">TEXT</option>
                        <option value="JSON">JSON</option>
                        <option value="PARQUET">PARQUET</option>
                      </select>
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">
                        Output Format
                      </label>
                      <select
                        name="outputType"
                        value={formData.outputType}
                        onChange={handleInputChange}
                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                      >
                        <option value="TEXT">TEXT</option>
                        <option value="JSON">JSON</option>
                        <option value="PARQUET">PARQUET</option>
                      </select>
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Output Directory
                    </label>
                    <input
                      type="text"
                      name="outputDir"
                      value={formData.outputDir}
                      onChange={handleInputChange}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="/output"
                      required
                    />
                  </div>

                  <div className="flex items-center">
                    <input
                      type="checkbox"
                      name="enableCombiner"
                      checked={formData.enableCombiner}
                      onChange={handleInputChange}
                      className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                    />
                    <label className="ml-2 text-sm font-medium text-gray-700">
                      Enable Combiner (for optimization)
                    </label>
                  </div>
                </div>
              </div>

              <div className="flex justify-end pb-7">
                <button
                  type="submit"
                  className="px-6 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                >
                  Submit Job
                </button>
              </div>
            </form>
          </div>
          
          {/* Upload */}
          <div className='flex max-w-3xl mx-5 mt-2'>
            <div className='w-full h-full flex flex-col justify-center'>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Choose File:
              </label>
              <div className='relative'>
                <input
                  id="fileInput"
                  type="file"
                  onChange={handleFileSelect}
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                />
                <div className='py-4 flex justify-center items-center border-dashed border-2 border-gray-800 rounded-lg hover:border-gray-600 transition-colors'>
                  <p className='text-lg text-gray-700'>
                    {uploadData.file ? uploadData.file.name : 'Click to Select File'}
                  </p>
                </div>
              </div>

              <div className='my-5 w-full'>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Upload Path:
                </label>
                <input 
                  type="text" 
                  value={uploadData.path}
                  onChange={handleUploadPathChange}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm"
                  placeholder="/data/myfile.csv"
                />
              </div>
              
              <button
                type="button"
                onClick={handleFileUpload}
                disabled={!uploadData.file || !uploadData.path}
                className="mt-4 px-6 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:bg-gray-400 disabled:cursor-not-allowed"
              >
                Upload
              </button>
            </div>
          </div>

        </div>
      </div>
      
      
    </div>
  );
};

export default StartJob;