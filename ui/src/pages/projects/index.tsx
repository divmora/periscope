import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { ProjectsAPI } from "../../api/projects";
import { Plus, Database, Activity, Server, Clock, HardDrive, Users } from "lucide-react";
import { useAuth } from "../../context/AuthContext";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";

export default function ProjectsPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const userRole = user?.global_role;

  const { data, isLoading } = useQuery({
    queryKey: ['projects'],
    queryFn: ProjectsAPI.listProjects,
  });

  const projects = data?.data || [];

  const handleNewProject = () => {
    alert("New Project creation flow to be implemented soon!");
  };

  return (
    <div className="flex-1 space-y-4 p-8 pt-6">
      <div className="flex items-center justify-between space-y-2">
        <h2 className="text-3xl font-bold tracking-tight">Dashboard</h2>
        <div className="flex items-center space-x-2">
          {userRole === "admin" && (
            <Button onClick={handleNewProject} className="bg-primary text-primary-foreground hover:bg-primary/90 shadow-md">
              <Plus className="mr-2 h-4 w-4" />
              New Project
            </Button>
          )}
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Projects</CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{projects.length}</div>
            <p className="text-xs text-muted-foreground">
              +0 from last month
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Storage Used</CardTitle>
            <HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">12.4 GB</div>
            <p className="text-xs text-muted-foreground">
              +2.1 GB from last month
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Syncs</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{projects.filter((p: any) => p.is_active).length || projects.length}</div>
            <p className="text-xs text-muted-foreground">
              Across all projects
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Team Members</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">3</div>
            <p className="text-xs text-muted-foreground">
              Administrators and Viewers
            </p>
          </CardContent>
        </Card>
      </div>

      <div className="mt-8">
        <h3 className="text-xl font-semibold text-foreground mb-4">Recent Projects</h3>
        
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {[1, 2, 3].map((i) => (
              <Card key={i} className="h-48 animate-pulse bg-muted/50" />
            ))}
          </div>
        ) : projects.length === 0 ? (
          <Card className="flex flex-col items-center justify-center p-16 text-center border-dashed">
            <Database className="h-10 w-10 text-muted-foreground mb-4" />
            <CardTitle className="mb-2">No projects found</CardTitle>
            <CardDescription className="mb-6 max-w-sm">
              Get started by creating a new Periscope project and connecting it to your database.
            </CardDescription>
            {userRole === "admin" && (
              <Button onClick={handleNewProject} variant="outline">
                <Plus className="mr-2 h-4 w-4" />
                Create your first project
              </Button>
            )}
          </Card>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {projects.map((project: any) => (
              <Card 
                key={project.iid}
                onClick={() => navigate(`/${project.iid}/telescope/requests`)}
                className="group relative flex flex-col justify-between hover:border-primary/50 hover:shadow-md transition-all cursor-pointer overflow-hidden"
              >
                <CardHeader className="pb-4">
                  <div className="flex items-start justify-between mb-2">
                    <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center group-hover:scale-110 transition-transform">
                      <Server className="h-5 w-5 text-primary" />
                    </div>
                    <span className={`px-2 py-0.5 rounded-full text-[10px] font-medium uppercase tracking-wider ${
                      project.prune_enabled ? 'bg-amber-500/10 text-amber-500 border border-amber-500/20' : 'bg-emerald-500/10 text-emerald-500 border border-emerald-500/20'
                    }`}>
                      {project.prune_enabled ? 'Pruning' : 'Archiving'}
                    </span>
                  </div>
                  <CardTitle className="text-lg group-hover:text-primary transition-colors">
                    {project.name}
                  </CardTitle>
                  <CardDescription className="flex items-center gap-2 mt-1">
                    <Database className="h-3 w-3" />
                    {project.storage_type.toUpperCase()} / {project.storage_bucket}
                  </CardDescription>
                </CardHeader>
                <CardFooter className="pt-4 border-t border-border bg-muted/20 flex items-center justify-between text-xs text-muted-foreground">
                  <div className="flex items-center gap-1.5">
                    <Activity className="h-3.5 w-3.5 text-blue-500" />
                    Syncing...
                  </div>
                  <div className="flex items-center gap-1.5">
                    <Clock className="h-3.5 w-3.5" />
                    {new Date(project.created_at).toLocaleDateString()}
                  </div>
                </CardFooter>
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
