import React from 'react';
import type { FileSystemNode } from '../hooks/useWebSocket';

interface FileBrowserProps {
  directory: FileSystemNode[] | null;
}

interface FileNodeProps {
  node: FileSystemNode;
  depth?: number;
}

const FileNode: React.FC<FileNodeProps> = ({ node, depth = 0 }) => {
  const indent = depth * 16; // 16px per level
  
  const getIcon = () => {
    if (node.type === 'directory') {
      return (
        <svg className="w-4 h-4 text-blue-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-5l-2-2H5a2 2 0 00-2 2z" />
        </svg>
      );
    } else {
      return (
        <svg className="w-4 h-4 text-gray-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
      );
    }
  };

  return (
    <div className="select-none">
      <div 
        className="flex items-center py-1 px-2 hover:bg-gray-100 rounded text-sm"
        style={{ paddingLeft: `${8 + indent}px` }}
      >
        {getIcon()}
        <span className="ml-2 truncate" title={node.name}>
          {node.name}
        </span>
      </div>
      
      {node.children && (
        <div>
          {node.children.map((child, index) => (
            <FileNode 
              key={`${child.path}-${index}`} 
              node={child} 
              depth={depth + 1} 
            />
          ))}
        </div>
      )}
    </div>
  );
};

const FileBrowser: React.FC<FileBrowserProps> = ({ directory }) => {
  return (
    <div className="flex flex-col h-full bg-white pt-18">
      <div className="border-b border-gray-200 p-2">
        <h3 className="text-lg font-semibold text-gray-800">Volume Files</h3>
      </div>
      <div className="overflow-y-auto h-full">
        {directory && directory.map((node, index) => (
          <FileNode key={`${node.path}-${index}`} node={node} depth={0} />
        ))}
      </div>
    </div>
  );
};

export default FileBrowser;