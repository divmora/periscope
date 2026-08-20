import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ProjectsAPI, Project, ProjectMember } from "../../api/projects";
import { 
  Save, Server, Database, Shield, AlertTriangle, Loader2, 
  Users, UserPlus, Trash2, ShieldCheck, ShieldAlert 
} from "lucide-react";

export default function SettingsPage() {
  const { projectId } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  
  // 1. Fetch Project Config
  const { data, isLoading } = useQuery({
    queryKey: ['project_config', projectId],
    queryFn: () => ProjectsAPI.getProjectConfig(projectId as string),
    enabled: !!projectId,
  });

  // 2. Fetch Project Members
  const { data: membersData, isLoading: isLoadingMembers } = useQuery({
    queryKey: ['project_members', projectId],
    queryFn: () => ProjectsAPI.listMembers(projectId as string),
    enabled: !!projectId,
  });

  const [formData, setFormData] = useState<Partial<Project>>({});
  const [successMsg, setSuccessMsg] = useState("");
  const [memberError, setMemberError] = useState("");
  const [memberSuccess, setMemberSuccess] = useState("");

  // Add Member Form State
  const [newMemberIdentifier, setNewMemberIdentifier] = useState("");
  const [newMemberRole, setNewMemberRole] = useState<"viewer" | "member" | "admin">("member");

  useEffect(() => {
    if (data?.data) {
      setFormData(data.data);
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (newConfig: Partial<Project>) => ProjectsAPI.updateProjectConfig(projectId as string, newConfig),
    onSuccess: () => {
      setSuccessMsg("Configuration saved successfully.");
      queryClient.invalidateQueries({ queryKey: ['project_config', projectId] });
      setTimeout(() => setSuccessMsg(""), 3000);
    }
  });

  const addMemberMutation = useMutation({
    mutationFn: () => ProjectsAPI.addMember(projectId as string, newMemberIdentifier, newMemberRole),
    onSuccess: () => {
      setMemberSuccess("Member added successfully.");
      setMemberError("");
      setNewMemberIdentifier("");
      queryClient.invalidateQueries({ queryKey: ['project_members', projectId] });
      setTimeout(() => setMemberSuccess(""), 3000);
    },
    onError: (err: any) => {
      setMemberError(err?.message || "Failed to add member");
    }
  });

  const updateRoleMutation = useMutation({
    mutationFn: ({ userIid, role }: { userIid: string; role: "viewer" | "member" | "admin" }) => 
      ProjectsAPI.updateMemberRole(projectId as string, userIid, role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project_members', projectId] });
    }
  });

  const removeMemberMutation = useMutation({
    mutationFn: (userIid: string) => ProjectsAPI.removeMember(projectId as string, userIid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project_members', projectId] });
    }
  });

  const deleteProjectMutation = useMutation({
    mutationFn: () => ProjectsAPI.deleteProject(projectId as string),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      navigate('/projects');
    }
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  const handleAddMember = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newMemberIdentifier.trim()) return;
    addMemberMutation.mutate();
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const value = e.target.type === 'checkbox' ? (e.target as HTMLInputElement).checked : e.target.value;
    setFormData({ ...formData, [e.target.name]: value });
  };

  if (isLoading) {
    return <div className="flex items-center justify-center h-full"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  }

  return (
    <div className="max-w-4xl mx-auto pb-16 pt-4">
      <div className="mb-8">
        <h1 className="text-2xl font-bold tracking-tight text-foreground mb-2">Project Configuration & Access</h1>
        <p className="text-sm text-muted-foreground">
          Manage database connections, storage destinations, data retention policies, and team permissions for this project.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-8">
        
        {/* General Settings */}
        <div className="bg-[#09090b] border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-[#18181b]/50 flex items-center gap-3">
            <Server className="h-5 w-5 text-primary" />
            <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">General Configuration</h2>
          </div>
          <div className="p-6 space-y-4">
            <div>
              <label className="block text-sm font-medium text-foreground mb-1.5">Project Name</label>
              <input 
                type="text" 
                name="name"
                value={formData.name || ""}
                onChange={handleChange}
                className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-foreground mb-1.5">Laravel Database DSN</label>
              <p className="text-xs text-muted-foreground mb-2">
                The connection string to your source Laravel MySQL/PostgreSQL database.
              </p>
              <input 
                type="password" 
                name="source_dsn"
                value={formData.source_dsn || ""}
                onChange={handleChange}
                placeholder="mysql://user:password@host:3306/laravel"
                className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                required
              />
            </div>
          </div>
        </div>

        {/* Storage Target */}
        <div className="bg-[#09090b] border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-[#18181b]/50 flex items-center gap-3">
            <Database className="h-5 w-5 text-emerald-500" />
            <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">Parquet Storage Target</h2>
          </div>
          <div className="p-6 space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Storage Type</label>
                <select 
                  name="storage_type"
                  value={formData.storage_type || "s3"}
                  onChange={handleChange}
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                >
                  <option value="s3">AWS S3</option>
                  <option value="minio">MinIO (S3 Compatible)</option>
                  <option value="r2">Cloudflare R2</option>
                  <option value="local">Local Disk</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Bucket Name</label>
                <input 
                  type="text" 
                  name="storage_bucket"
                  value={formData.storage_bucket || ""}
                  onChange={handleChange}
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                  required
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Region (Optional)</label>
                <input 
                  type="text" 
                  name="storage_region"
                  value={formData.storage_region || ""}
                  onChange={handleChange}
                  placeholder="us-east-1"
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Custom Endpoint (Optional)</label>
                <input 
                  type="text" 
                  name="storage_endpoint"
                  value={formData.storage_endpoint || ""}
                  onChange={handleChange}
                  placeholder="https://minio.local:9000"
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4 pt-2">
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Access Key ID</label>
                <input 
                  type="password" 
                  name="storage_access_key"
                  value={formData.storage_access_key || ""}
                  onChange={handleChange}
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Secret Access Key</label>
                <input 
                  type="password" 
                  name="storage_secret_key"
                  value={formData.storage_secret_key || ""}
                  onChange={handleChange}
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                />
              </div>
            </div>
          </div>
        </div>

        {/* Data Retention */}
        <div className="bg-[#09090b] border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-[#18181b]/50 flex items-center gap-3">
            <Shield className="h-5 w-5 text-amber-500" />
            <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">Data Retention & Pruning</h2>
          </div>
          <div className="p-6">
            <div className="flex items-start gap-4 p-4 rounded-lg bg-amber-500/10 border border-amber-500/20">
              <AlertTriangle className="h-5 w-5 text-amber-500 mt-0.5" />
              <div>
                <label className="flex items-center gap-2 text-sm font-medium text-amber-500 mb-1">
                  <input 
                    type="checkbox" 
                    name="prune_enabled"
                    checked={formData.prune_enabled || false}
                    onChange={handleChange}
                    className="rounded border-border bg-[#18181b] text-amber-500 focus:ring-amber-500 focus:ring-offset-[#09090b]"
                  />
                  Destructive Sync (Prune Source Database)
                </label>
                <p className="text-xs text-amber-500/80">
                  If enabled, Periscope will automatically delete Telescope entries from your production database immediately after they are securely backed up to Parquet storage. This prevents your operational database from bloating.
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Save Bar */}
        <div className="flex items-center justify-between py-2">
          <p className="text-sm text-emerald-500 font-medium">{successMsg}</p>
          <button 
            type="submit" 
            disabled={mutation.isPending}
            className="flex items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 px-6 py-2.5 rounded-md text-sm font-medium transition-colors shadow-sm disabled:opacity-50 cursor-pointer"
          >
            {mutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            Save Configuration
          </button>
        </div>
      </form>

      {/* Team & RBAC Access Control */}
      {projectId && (
        <div className="mt-12 bg-[#09090b] border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-[#18181b]/50 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Users className="h-5 w-5 text-blue-400" />
              <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">Team & Access Control (RBAC)</h2>
            </div>
          </div>

          <div className="p-6 space-y-6">
            {/* Add Member Form */}
            <form onSubmit={handleAddMember} className="flex flex-col sm:flex-row gap-3 items-end">
              <div className="flex-1 w-full">
                <label className="block text-xs font-medium text-foreground mb-1.5">User Email or Username</label>
                <input 
                  type="text" 
                  value={newMemberIdentifier}
                  onChange={(e) => setNewMemberIdentifier(e.target.value)}
                  placeholder="colleague@example.com"
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                  required
                />
              </div>
              <div className="w-full sm:w-40">
                <label className="block text-xs font-medium text-foreground mb-1.5">Project Role</label>
                <select 
                  value={newMemberRole}
                  onChange={(e) => setNewMemberRole(e.target.value as "viewer" | "member" | "admin")}
                  className="w-full bg-[#18181b] border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                >
                  <option value="viewer">Viewer (Read-only)</option>
                  <option value="member">Member (Sync & Query)</option>
                  <option value="admin">Admin (Full Control)</option>
                </select>
              </div>
              <button 
                type="submit" 
                disabled={addMemberMutation.isPending}
                className="flex items-center justify-center gap-2 bg-blue-600 text-white hover:bg-blue-500 px-4 py-2 rounded-md text-sm font-medium transition-colors disabled:opacity-50 cursor-pointer h-[38px] w-full sm:w-auto"
              >
                {addMemberMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <UserPlus className="h-4 w-4" />}
                Add Member
              </button>
            </form>

            {memberError && <p className="text-xs text-rose-500 font-medium">{memberError}</p>}
            {memberSuccess && <p className="text-xs text-emerald-500 font-medium">{memberSuccess}</p>}

            {/* Members Table */}
            <div className="border border-border rounded-lg overflow-hidden">
              <table className="w-full text-left text-sm">
                <thead className="bg-[#18181b] text-muted-foreground text-xs uppercase border-b border-border">
                  <tr>
                    <th className="px-4 py-3">Member</th>
                    <th className="px-4 py-3">Role</th>
                    <th className="px-4 py-3">Joined</th>
                    <th className="px-4 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {isLoadingMembers ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-6 text-center text-muted-foreground">
                        <Loader2 className="h-5 w-5 animate-spin mx-auto text-primary" />
                      </td>
                    </tr>
                  ) : membersData?.data && membersData.data.length > 0 ? (
                    membersData.data.map((member: ProjectMember) => (
                      <tr key={member.user_iid} className="hover:bg-muted/10 transition-colors">
                        <td className="px-4 py-3">
                          <div className="font-medium text-foreground">{member.username}</div>
                          <div className="text-xs text-muted-foreground">{member.email}</div>
                        </td>
                        <td className="px-4 py-3">
                          <select 
                            value={member.role}
                            onChange={(e) => updateRoleMutation.mutate({ 
                              userIid: member.user_iid, 
                              role: e.target.value as "viewer" | "member" | "admin" 
                            })}
                            className="bg-[#18181b] border border-border rounded px-2 py-1 text-xs text-foreground focus:outline-none focus:border-primary capitalize cursor-pointer"
                          >
                            <option value="viewer">Viewer</option>
                            <option value="member">Member</option>
                            <option value="admin">Admin</option>
                          </select>
                        </td>
                        <td className="px-4 py-3 text-xs text-muted-foreground">
                          {new Date(member.created_at).toLocaleDateString()}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button 
                            onClick={() => removeMemberMutation.mutate(member.user_iid)}
                            className="text-muted-foreground hover:text-destructive p-1 rounded transition-colors cursor-pointer"
                            title="Remove member"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan={4} className="px-4 py-6 text-center text-muted-foreground text-xs">
                        No team members configured for this project.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Danger Zone */}
      {projectId && (
        <div className="mt-12 bg-[#09090b] border border-destructive/30 rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-destructive/30 bg-destructive/10 flex items-center gap-3">
            <ShieldAlert className="h-5 w-5 text-destructive" />
            <h2 className="text-sm font-semibold text-destructive uppercase tracking-wider">Danger Zone</h2>
          </div>
          <div className="p-6 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div>
              <h3 className="text-sm font-medium text-foreground">Delete this Project</h3>
              <p className="text-xs text-muted-foreground mt-1">
                Once deleted, this project configuration and its local metadata will be permanently removed.
              </p>
            </div>
            <button 
              onClick={() => {
                if (window.confirm("Are you sure you want to delete this project? This action cannot be undone.")) {
                  deleteProjectMutation.mutate();
                }
              }}
              disabled={deleteProjectMutation.isPending}
              className="flex items-center gap-2 bg-destructive text-destructive-foreground hover:bg-destructive/90 px-4 py-2 rounded-md text-sm font-medium transition-colors disabled:opacity-50 cursor-pointer whitespace-nowrap"
            >
              {deleteProjectMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
              Delete Project
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
