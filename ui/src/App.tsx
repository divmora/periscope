import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { AuthProvider } from "./context/AuthContext"
import RootLayout from "./layouts/RootLayout"
import ProjectsPage from "./pages/projects"
import SettingsPage from "./pages/settings"
import LoginPage from "./pages/login"
import RequestsPage from "./pages/requests"
import ExceptionsPage from "./pages/exceptions"
import QueriesPage from "./pages/queries"
import LogsPage from "./pages/logs"
import SourcesPage from "./pages/sources/SourcesPage"
import SQLStudioPage from "./pages/studio/SQLStudioPage"
import DASPage from "./pages/das"
import ProtectedRoute from "./components/ProtectedRoute"

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          
          {/* Protected Routes */}
          <Route element={<ProtectedRoute />}>
            <Route element={<RootLayout />}>
              <Route path="/" element={<Navigate to="/projects" replace />} />
              <Route path="/projects" element={<ProjectsPage />} />
              <Route path="/settings" element={<SettingsPage />} />

              <Route path="/:projectId">
                <Route index element={<Navigate to="sources" replace />} />
                <Route path="sources" element={<SourcesPage />} />
                <Route path="studio" element={<SQLStudioPage />} />
                <Route path="das" element={<DASPage />} />
                <Route path="config" element={<SettingsPage />} />
                
                <Route path="telescope/requests" element={<RequestsPage />} />
                <Route path="telescope/exceptions" element={<ExceptionsPage />} />
                <Route path="telescope/queries" element={<QueriesPage />} />
                <Route path="telescope/logs" element={<LogsPage />} />
              </Route>
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}


export default App
