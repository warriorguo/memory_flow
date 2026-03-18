import { createBrowserRouter, Navigate } from 'react-router-dom';
import AppLayout from './components/Layout/AppLayout';
import ProjectList from './pages/ProjectList';
import ProjectDetail from './pages/ProjectDetail';
import IssueDetail from './pages/IssueDetail';
import MemoryList from './pages/MemoryList';

const router = createBrowserRouter([
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <Navigate to="/projects" replace /> },
      { path: 'projects', element: <ProjectList /> },
      { path: 'projects/:projectId', element: <ProjectDetail /> },
      { path: 'issues/:id', element: <IssueDetail /> },
      { path: 'memories', element: <MemoryList /> },
    ],
  },
]);

export default router;
