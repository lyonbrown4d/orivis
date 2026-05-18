import { Navigate, Route, Routes } from 'react-router-dom';
import DashboardPage from '@/pages/DashboardPage';
import LoginPage from '@/pages/LoginPage';
import MonitorDetailPage from '@/pages/MonitorDetailPage';

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<DashboardPage />} />
      <Route path="/monitors/:monitorId" element={<MonitorDetailPage />} />
      <Route path="/:group" element={<DashboardPage statusPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
