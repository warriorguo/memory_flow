import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu } from 'antd';
import {
  ProjectOutlined,
  BulbOutlined,
  LogoutOutlined,
} from '@ant-design/icons';

const { Sider } = Layout;

interface SidebarProps {
  onLogout: () => void;
}

const Sidebar: React.FC<SidebarProps> = ({ onLogout }) => {
  const navigate = useNavigate();
  const location = useLocation();

  const menuItems = [
    {
      key: '/projects',
      icon: <ProjectOutlined />,
      label: '项目管理',
    },
    {
      key: '/memories',
      icon: <BulbOutlined />,
      label: 'Memory 管理',
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      danger: true,
    },
  ];

  const handleClick = ({ key }: { key: string }) => {
    if (key === 'logout') {
      onLogout();
      navigate('/login');
    } else {
      navigate(key);
    }
  };

  return (
    <Sider width={220} theme="light" style={{ borderRight: '1px solid #f0f0f0' }}>
      <div style={{ height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 'bold', fontSize: 18 }}>
        Memory Flow
      </div>
      <Menu
        mode="inline"
        selectedKeys={[location.pathname.startsWith('/projects') ? '/projects' : location.pathname.startsWith('/memories') ? '/memories' : '']}
        items={menuItems}
        onClick={handleClick}
      />
    </Sider>
  );
};

export default Sidebar;
