import React from 'react';
import { Outlet, Navigate } from 'react-router-dom';
import { Layout } from 'antd';
import Sidebar from './Sidebar';
import { useAuth } from '../../hooks/useAuth';

const { Content } = Layout;

const AppLayout: React.FC = () => {
  const { user, logout } = useAuth();

  if (!user) return <Navigate to="/login" replace />;

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar onLogout={logout} />
      <Layout>
        <Content style={{ padding: 24, background: '#f5f5f5' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
};

export default AppLayout;
