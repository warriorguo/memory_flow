import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu } from 'antd';
import {
  ProjectOutlined,
  BulbOutlined,
} from '@ant-design/icons';

const { Sider } = Layout;

const Sidebar: React.FC = () => {
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
  ];

  const handleClick = ({ key }: { key: string }) => {
    navigate(key);
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
