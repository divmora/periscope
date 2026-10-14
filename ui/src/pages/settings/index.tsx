import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ProjectsAPI, Project, ProjectMember } from "../../api/projects";
import { SourcesAPI } from "../../api/sources";
import { SystemAPI } from "../../api/system";
import { TeamsManager } from "./TeamsManager";
import { 
  Save, Server, Database, Loader2, 
  Users, UserPlus, Trash2, ShieldCheck, ShieldAlert, HardDrive, 
  Key, Cpu, CheckCircle2, Sparkles, UserCheck
} from "lucide-react";

export default function SettingsPage() {
  const { projectId } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [rbacTab, setRbacTab] = useState<"teams" | "members">("teams");

  // Global system storage query
  const { data: systemStorage } = useQuery({
    queryKey: ['system_storage'],
    queryFn: SystemAPI.getSystemStorage,
  });
  
  // 1. Fetch Project Config (if inside project)
  const { data, isLoading } = useQuery({
    queryKey: ['project_config', projectId],
    queryFn: () => ProjectsAPI.getProjectConfig(projectId as string),
    enabled: !!projectId,
  });

  // 2. Fetch Project Members (if inside project)
  const { data: membersData, isLoading: isLoadingMembers } = useQuery({
    queryKey: ['project_members', projectId],
    queryFn: () => ProjectsAPI.listMembers(projectId as string),
    enabled: !!projectId,
  });

  // 3. Fetch Project Sources (for granular RBAC permissions)
  const { data: sourcesData } = useQuery({
    queryKey: ['sources', projectId],
    queryFn: () => SourcesAPI.listSources(projectId as string),
    enabled: !!projectId,
  });

  const sources = sourcesData?.data || [];

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
    return <div className="flex items-center justify-center h-full p-12"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  }

  // If viewing global /settings (not scoped to a specific project)
  if (!projectId) {
    return (
      <div className="max-w-4xl mx-auto pb-16 pt-6 px-4 space-y-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground mb-1">Global System Configuration</h1>
          <p className="text-sm text-muted-foreground">
            Platform-wide storage clusters, cryptographic keys, and metadata services.
          </p>
        </div>

        {/* Global Cluster Storage */}
        <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-muted/30 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <HardDrive className="h-5 w-5 text-emerald-500" />
              <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">Cluster Default Storage Target</h2>
            </div>
            <span className="flex items-center gap-1.5 text-xs font-medium text-emerald-400 bg-emerald-500/10 px-2.5 py-0.5 rounded-full border border-emerald-500/20">
              <CheckCircle2 className="h-3.5 w-3.5" />
              Online &amp; Connected
            </span>
          </div>
          <div className="p-6 space-y-4">
            <p className="text-xs text-muted-foreground leading-relaxed">
              New telemetry projects automatically inherit this default S3 / RustFS cluster storage unless custom bucket overrides are configured.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 pt-2">
              <div className="p-3.5 rounded-lg border border-border bg-muted/20">
                <span className="text-[11px] text-muted-foreground uppercase font-medium">Default Bucket</span>
                <p className="text-sm font-semibold text-foreground font-mono mt-1">
                  {systemStorage?.data?.storage_bucket || "periscope-telemetry"}
                </p>
              </div>
              <div className="p-3.5 rounded-lg border border-border bg-muted/20">
                <span className="text-[11px] text-muted-foreground uppercase font-medium">Storage Engine</span>
                <p className="text-sm font-semibold text-foreground font-mono mt-1">
                  RustFS S3 Gateway
                </p>
              </div>
              <div className="p-3.5 rounded-lg border border-border bg-muted/20">
                <span className="text-[11px] text-muted-foreground uppercase font-medium">Compression</span>
                <p className="text-sm font-semibold text-foreground font-mono mt-1">
                  ZSTD (Apache Parquet)
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Security & Cryptography */}
        <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-muted/30 flex items-center gap-3">
            <ShieldCheck className="h-5 w-5 text-primary" />
            <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">Security &amp; Encryption</h2>
          </div>
          <div className="p-6 space-y-3 text-xs text-muted-foreground">
            <div className="flex items-center justify-between py-2 border-b border-border/60">
              <span className="flex items-center gap-2 text-foreground font-medium">
                <Key className="h-4 w-4 text-primary" />
                Database DSN &amp; S3 Credentials Encryption
              </span>
              <span className="font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">AES-256-GCM Active</span>
            </div>
            <div className="flex items-center justify-between py-2 border-b border-border/60">
              <span className="flex items-center gap-2 text-foreground font-medium">
                <Cpu className="h-4 w-4 text-blue-400" />
                Analytical Query Processor
              </span>
              <span className="font-mono text-blue-400 bg-blue-500/10 px-2 py-0.5 rounded border border-blue-500/20">Embedded DuckDB v1.8</span>
            </div>
            <div className="flex items-center justify-between py-2">
              <span className="flex items-center gap-2 text-foreground font-medium">
                <Database className="h-4 w-4 text-amber-400" />
                Metadata Catalog Database
              </span>
              <span className="font-mono text-foreground">PostgreSQL 17+ (pgx pool)</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Project-scoped settings
  return (
    <div className="max-w-4xl mx-auto pb-16 pt-4 px-4 space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground mb-1">Project Configuration &amp; Access</h1>
        <p className="text-sm text-muted-foreground">
          Manage database connections, storage destinations, data retention policies, and team permissions for this project.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-8">
        
        {/* General Settings */}
        <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-muted/30 flex items-center gap-3">
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
                className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-foreground mb-1.5">Primary Laravel Database DSN (Optional)</label>
              <p className="text-xs text-muted-foreground mb-2">
                Connection string for Laravel Telescope sync. Stored securely with AES-256-GCM encryption.
              </p>
              <input 
                type="password" 
                name="source_dsn"
                value={formData.source_dsn || ""}
                onChange={handleChange}
                placeholder="mysql://user:password@host:3306/laravel"
                className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
              />
            </div>
          </div>
        </div>

        {/* Data Retention & Lifecycle Tiering */}
        <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-muted/30 flex items-center gap-3">
            <Sparkles className="h-5 w-5 text-amber-500" />
            <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">Data Retention &amp; Lifecycle Tiering</h2>
          </div>
          <div className="p-6 space-y-5">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Source DB Pruning Policy</label>
                <select
                  name="source_retention_days"
                  value={formData.source_retention_days ?? 0}
                  onChange={(e) => setFormData({ ...formData, source_retention_days: Number(e.target.value) })}
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                >
                  <option value="0">Prune immediately upon Parquet commit</option>
                  <option value="7">Retain last 7 days in source DB</option>
                  <option value="30">Retain last 30 days in source DB</option>
                  <option value="-1">Never prune source DB (Read-only sync)</option>
                </select>
                <p className="text-xs text-muted-foreground mt-1.5">
                  Applies to Periscope-managed collectors (e.g. Telescope). Controls when processed entries are safely drained from your operational database.
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Parquet Lakehouse Retention</label>
                <select
                  name="lakehouse_retention_days"
                  value={formData.lakehouse_retention_days ?? 365}
                  onChange={(e) => setFormData({ ...formData, lakehouse_retention_days: Number(e.target.value) })}
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                >
                  <option value="30">30 Days</option>
                  <option value="90">90 Days</option>
                  <option value="365">1 Year (365 Days)</option>
                  <option value="1095">3 Years</option>
                  <option value="0">Indefinite (Keep Forever)</option>
                </select>
                <p className="text-xs text-muted-foreground mt-1.5">
                  Applies to Parquet files generated by Periscope. External sources (Aurora DAS, Athena) manage their own S3 lifecycle and retention policies in AWS.
                </p>
              </div>
            </div>

            <div className="flex items-center justify-between p-4 rounded-lg border border-border bg-muted/20">
              <div>
                <span className="text-sm font-medium text-foreground block">Automatic Parquet Compaction</span>
                <p className="text-xs text-muted-foreground mt-0.5">
                  Periodically compact small hourly Parquet chunks into daily consolidated files for faster DuckDB queries.
                </p>
              </div>
              <input 
                type="checkbox" 
                name="compaction_enabled"
                checked={formData.compaction_enabled ?? true}
                onChange={handleChange}
                className="h-4 w-4 rounded border-border text-primary focus:ring-primary cursor-pointer"
              />
            </div>
          </div>
        </div>

        {/* Storage Target */}
        <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-border bg-muted/30 flex items-center gap-3">
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
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
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
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
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
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
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
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
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
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-foreground mb-1.5">Secret Access Key</label>
                <input 
                  type="password" 
                  name="storage_secret_key"
                  value={formData.storage_secret_key || ""}
                  onChange={handleChange}
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                />
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

      {/* Team & RBAC Access Control (Separate Form) */}
      <div className="mt-12 bg-card border border-border rounded-xl overflow-hidden shadow-sm">
        <div className="px-6 py-4 border-b border-border bg-muted/30 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <Users className="h-5 w-5 text-blue-400" />
            <div>
              <h2 className="text-sm font-semibold text-foreground uppercase tracking-wider">Access &amp; Permissions (RBAC)</h2>
              <p className="text-xs text-muted-foreground">Manage project teams, roles, and granular data source access.</p>
            </div>
          </div>

          {/* Tab Switcher: Teams vs Direct Members */}
          <div className="flex items-center bg-muted/60 border border-border p-1 rounded-lg">
            <button
              type="button"
              onClick={() => setRbacTab("teams")}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-all ${
                rbacTab === "teams"
                  ? "bg-card text-foreground shadow-sm border border-border"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <Users className="h-3.5 w-3.5 text-primary" />
              Teams (Recommended)
            </button>
            <button
              type="button"
              onClick={() => setRbacTab("members")}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-all ${
                rbacTab === "members"
                  ? "bg-card text-foreground shadow-sm border border-border"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <UserCheck className="h-3.5 w-3.5 text-blue-400" />
              Direct Members
            </button>
          </div>
        </div>

        <div className="p-6">
          {rbacTab === "teams" ? (
            <TeamsManager projectId={projectId as string} sources={sources} />
          ) : (
            <div className="space-y-6">
          <form onSubmit={handleAddMember} className="flex flex-col sm:flex-row gap-3 items-end">
            <div className="flex-1 w-full">
              <label className="block text-xs font-medium text-foreground mb-1.5">User Email or Username</label>
              <input 
                type="text" 
                value={newMemberIdentifier}
                onChange={(e) => setNewMemberIdentifier(e.target.value)}
                placeholder="colleague@example.com"
                className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
                required
              />
            </div>
            <div className="w-full sm:w-40">
              <label className="block text-xs font-medium text-foreground mb-1.5">Project Role</label>
              <select 
                value={newMemberRole}
                onChange={(e) => setNewMemberRole(e.target.value as "viewer" | "member" | "admin")}
                className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary transition-colors"
              >
                <option value="viewer">Viewer (Read-only)</option>
                <option value="member">Member (Sync &amp; Query)</option>
                <option value="admin">Admin (Full Control)</option>
              </select>
            </div>
            <button 
              type="submit" 
              disabled={addMemberMutation.isPending}
              className="flex items-center justify-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 px-4 py-2 rounded-md text-sm font-medium transition-colors disabled:opacity-50 cursor-pointer h-[38px] w-full sm:w-auto"
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
              <thead className="bg-muted/50 text-muted-foreground text-xs uppercase border-b border-border">
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
                          className="bg-muted/50 border border-border rounded px-2 py-1 text-xs text-foreground focus:outline-none focus:border-primary capitalize cursor-pointer"
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
      )}
    </div>
  </div>

      {/* Danger Zone */}
      <div className="mt-12 bg-card border border-destructive/30 rounded-xl overflow-hidden shadow-sm">
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
    </div>
  );
}
