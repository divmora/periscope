import { useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useMutation } from "@tanstack/react-query";
import { ProjectsAPI, Project } from "../../api/projects";
import { Save, Server, Database, Shield, AlertTriangle, Loader2 } from "lucide-react";

export default function SettingsPage() {
  const { projectId } = useParams();
  
  const { data, isLoading } = useQuery({
    queryKey: ['project_config', projectId],
    queryFn: () => ProjectsAPI.getProjectConfig(projectId as string),
    enabled: !!projectId,
  });

  const [formData, setFormData] = useState<Partial<Project>>({});
  const [successMsg, setSuccessMsg] = useState("");

  useEffect(() => {
    if (data?.data) {
      setFormData(data.data);
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (newConfig: Partial<Project>) => ProjectsAPI.updateProjectConfig(projectId as string, newConfig),
    onSuccess: () => {
      setSuccessMsg("Configuration saved successfully.");
      setTimeout(() => setSuccessMsg(""), 3000);
    }
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const value = e.target.type === 'checkbox' ? (e.target as HTMLInputElement).checked : e.target.value;
    setFormData({ ...formData, [e.target.name]: value });
  };

  if (isLoading) {
    return <div className="flex items-center justify-center h-full"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  }

  return (
    <div className="max-w-4xl mx-auto pb-12">
      <div className="mb-8">
        <h1 className="text-2xl font-bold tracking-tight text-foreground mb-2">Project Configuration</h1>
        <p className="text-sm text-muted-foreground">
          Manage database connections, storage destinations, and data retention policies for this project.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-8">
        
        {/* General Settings */}
        <div className="bg-[#09090b] border border-border rounded-xl overflow-hidden">
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
        <div className="bg-[#09090b] border border-border rounded-xl overflow-hidden">
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
        <div className="bg-[#09090b] border border-border rounded-xl overflow-hidden">
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
            className="flex items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 px-6 py-2.5 rounded-md text-sm font-medium transition-colors shadow-[0_0_15px_rgba(var(--primary),0.3)] disabled:opacity-50"
          >
            {mutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            Save Configuration
          </button>
        </div>

      </form>
    </div>
  );
}
