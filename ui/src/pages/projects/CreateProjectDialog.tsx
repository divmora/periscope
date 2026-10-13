import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ProjectsAPI } from "../../api/projects";
import { SystemAPI } from "../../api/system";
import { Button } from "@/components/ui/button";
import { Plus, Database, Cloud, HardDrive, ShieldCheck, Layers, Sparkles } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";

const formSchema = z.object({
  name: z.string().min(2, "Project name must be at least 2 characters"),
  source_type: z.string(),
  source_dsn: z.string().optional(),
  storage_mode: z.enum(["default", "custom"]),
  storage_type: z.string(),
  storage_bucket: z.string().optional(),
  storage_region: z.string().optional(),
  storage_endpoint: z.string().optional(),
  storage_access_key: z.string().optional(),
  storage_secret_key: z.string().optional(),
  source_retention_days: z.number(),
  lakehouse_retention_days: z.number(),
  compaction_enabled: z.boolean(),
  prune_enabled: z.boolean(),
});

export function CreateProjectDialog() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();

  const { data: systemStorage } = useQuery({
    queryKey: ['system_storage'],
    queryFn: SystemAPI.getSystemStorage,
  });

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      source_type: "laravel_telescope",
      source_dsn: "",
      storage_mode: "default",
      storage_type: "s3",
      storage_bucket: "",
      storage_region: "us-east-1",
      storage_endpoint: "",
      storage_access_key: "",
      storage_secret_key: "",
      source_retention_days: 0,
      lakehouse_retention_days: 365,
      compaction_enabled: true,
      prune_enabled: false,
    },
  });

  const storageMode = form.watch("storage_mode");
  const sourceType = form.watch("source_type");

  const createMutation = useMutation({
    mutationFn: (values: z.infer<typeof formSchema>) => {
      const payload: any = {
        name: values.name,
        source_retention_days: Number(values.source_retention_days),
        lakehouse_retention_days: Number(values.lakehouse_retention_days),
        compaction_enabled: values.compaction_enabled,
        prune_enabled: values.source_retention_days >= 0,
      };

      if (values.source_dsn?.trim()) {
        payload.source_dsn = values.source_dsn.trim();
      }

      if (values.storage_mode === "default") {
        payload.storage_bucket = "default";
        payload.storage_type = "s3";
      } else {
        payload.storage_type = values.storage_type || "s3";
        payload.storage_bucket = values.storage_bucket;
        payload.storage_region = values.storage_region;
        payload.storage_endpoint = values.storage_endpoint;
        payload.storage_access_key = values.storage_access_key;
        payload.storage_secret_key = values.storage_secret_key;
      }

      return ProjectsAPI.createProject(payload);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setOpen(false);
      form.reset();
    },
  });

  function onSubmit(values: z.infer<typeof formSchema>) {
    createMutation.mutate(values);
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button className="bg-primary text-primary-foreground hover:bg-primary/90 shadow-md">
          <Plus className="mr-2 h-4 w-4" />
          New Project
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto bg-card border-border">
        <DialogHeader>
          <DialogTitle className="text-2xl flex items-center gap-2 text-foreground">
            <Database className="h-5 w-5 text-primary" />
            Create Telemetry Project
          </DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Create an isolated lakehouse workspace for telemetry, audit streams, traces, and DuckDB OLAP queries.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
            
            {/* General Workspace Section */}
            <div className="space-y-4">
              <div className="flex items-center gap-2 pb-2 border-b border-border">
                <Database className="h-4 w-4 text-primary" />
                <h3 className="font-semibold text-sm text-foreground">General Workspace</h3>
              </div>
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Project Name</FormLabel>
                    <FormControl>
                      <Input placeholder="e.g. Production Analytics" {...field} />
                    </FormControl>
                    <FormDescription>
                      Unique workspace identifier for isolating your telemetry partitions.
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            {/* Ingestion Source Section */}
            <div className="space-y-4">
              <div className="flex items-center gap-2 pb-2 border-b border-border">
                <Layers className="h-4 w-4 text-blue-500" />
                <h3 className="font-semibold text-sm text-foreground">Initial Ingestion Source</h3>
              </div>

              <FormField
                control={form.control}
                name="source_type"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Source Type</FormLabel>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                      {[
                        { id: "laravel_telescope", label: "Telescope", desc: "Laravel Telemetry" },
                        { id: "aurora_das", label: "Aurora DAS", desc: "AWS Activity Stream" },
                        { id: "postgres_audit", label: "PG Audit", desc: "PostgreSQL Logs" },
                        { id: "none", label: "None / OTLP", desc: "Configure Later" },
                      ].map((s) => (
                        <div
                          key={s.id}
                          onClick={() => field.onChange(s.id)}
                          className={`cursor-pointer rounded-lg border p-3 text-left transition-all ${
                            field.value === s.id
                              ? "border-primary bg-primary/10 text-foreground"
                              : "border-border bg-muted/30 hover:bg-muted/50 text-muted-foreground"
                          }`}
                        >
                          <div className="text-xs font-semibold text-foreground">{s.label}</div>
                          <div className="text-[10px] text-muted-foreground mt-0.5">{s.desc}</div>
                        </div>
                      ))}
                    </div>
                  </FormItem>
                )}
              />

              {sourceType !== "none" && (
                <FormField
                  control={form.control}
                  name="source_dsn"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Database Connection String (DSN)</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder={
                            sourceType === "laravel_telescope"
                              ? "mysql://user:pass@host:3306/db or postgres://..."
                              : "postgres://user:pass@host:5432/db"
                          }
                          {...field}
                        />
                      </FormControl>
                      <FormDescription className="flex items-center gap-1 text-xs">
                        <ShieldCheck className="h-3.5 w-3.5 text-emerald-500" />
                        Credentials are automatically encrypted at rest with AES-256-GCM.
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </div>

            {/* Parquet Storage Section */}
            <div className="space-y-4">
              <div className="flex items-center gap-2 pb-2 border-b border-border">
                <Cloud className="h-4 w-4 text-emerald-500" />
                <h3 className="font-semibold text-sm text-foreground">Parquet Lakehouse Storage Target</h3>
              </div>

              <FormField
                control={form.control}
                name="storage_mode"
                render={({ field }) => (
                  <FormItem>
                    <div className="grid grid-cols-2 gap-3">
                      <div
                        onClick={() => field.onChange("default")}
                        className={`cursor-pointer rounded-lg border p-3.5 transition-all ${
                          field.value === "default"
                            ? "border-emerald-500 bg-emerald-500/10"
                            : "border-border bg-muted/30 hover:bg-muted/50"
                        }`}
                      >
                        <div className="flex items-center gap-2">
                          <HardDrive className="h-4 w-4 text-emerald-500" />
                          <div className="text-sm font-medium text-foreground">Default Storage</div>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 font-semibold">Recommended</span>
                        </div>
                        <p className="text-xs text-muted-foreground mt-1.5">
                          Use central cluster storage ({systemStorage?.data?.storage_bucket || "periscope-telemetry"}). No credentials required.
                        </p>
                      </div>

                      <div
                        onClick={() => field.onChange("custom")}
                        className={`cursor-pointer rounded-lg border p-3.5 transition-all ${
                          field.value === "custom"
                            ? "border-primary bg-primary/10"
                            : "border-border bg-muted/30 hover:bg-muted/50"
                        }`}
                      >
                        <div className="flex items-center gap-2">
                          <Cloud className="h-4 w-4 text-primary" />
                          <div className="text-sm font-medium text-foreground">Custom S3 / R2 Bucket</div>
                        </div>
                        <p className="text-xs text-muted-foreground mt-1.5">
                          Specify a dedicated AWS S3, Cloudflare R2, or custom MinIO bucket.
                        </p>
                      </div>
                    </div>
                  </FormItem>
                )}
              />

              {storageMode === "custom" && (
                <div className="space-y-4 pt-2">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="storage_bucket"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Bucket Name</FormLabel>
                          <FormControl>
                            <Input placeholder="my-telemetry-bucket" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="storage_region"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Region</FormLabel>
                          <FormControl>
                            <Input placeholder="us-east-1" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormField
                    control={form.control}
                    name="storage_endpoint"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Custom Endpoint (Optional)</FormLabel>
                        <FormControl>
                          <Input placeholder="https://s3.us-west-004.backblazeb2.com" {...field} />
                        </FormControl>
                        <FormDescription>Leave blank for standard AWS S3.</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="storage_access_key"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Access Key ID</FormLabel>
                          <FormControl>
                            <Input type="password" placeholder="AKIA..." {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="storage_secret_key"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Secret Access Key</FormLabel>
                          <FormControl>
                            <Input type="password" placeholder="••••••••" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </div>
              )}
            </div>

            {/* Lifecycle Tiering & Retention */}
            <div className="space-y-4">
              <div className="flex items-center gap-2 pb-2 border-b border-border">
                <Sparkles className="h-4 w-4 text-amber-500" />
                <h3 className="font-semibold text-sm text-foreground">Data Retention &amp; Lifecycle Tiering</h3>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <FormField
                  control={form.control}
                  name="source_retention_days"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Source DB Pruning</FormLabel>
                      <FormControl>
                        <select
                          value={field.value}
                          onChange={(e) => field.onChange(Number(e.target.value))}
                          className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary"
                        >
                          <option value="0">Prune immediately after Parquet commit</option>
                          <option value="7">Keep last 7 days in source DB</option>
                          <option value="30">Keep last 30 days in source DB</option>
                          <option value="-1">Never prune source DB (Read-only sync)</option>
                        </select>
                      </FormControl>
                      <FormDescription className="text-xs">
                        Controls when source entries are safely deleted.
                      </FormDescription>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="lakehouse_retention_days"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Parquet Cold Retention</FormLabel>
                      <FormControl>
                        <select
                          value={field.value}
                          onChange={(e) => field.onChange(Number(e.target.value))}
                          className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary"
                        >
                          <option value="30">30 Days</option>
                          <option value="90">90 Days</option>
                          <option value="365">1 Year (365 Days)</option>
                          <option value="1095">3 Years</option>
                          <option value="0">Indefinite (Keep Forever)</option>
                        </select>
                      </FormControl>
                      <FormDescription className="text-xs">
                        Duration before cold storage partitions are retired.
                      </FormDescription>
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name="compaction_enabled"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between rounded-lg border border-border p-3.5 bg-muted/20">
                    <div className="space-y-0.5">
                      <FormLabel className="text-sm font-medium">Automatic Parquet Compaction</FormLabel>
                      <FormDescription className="text-xs">
                        Periodically compact small hourly Parquet chunks into daily optimized files for maximum DuckDB query speed.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button
                type="submit"
                className="bg-primary text-primary-foreground hover:bg-primary/90"
                disabled={createMutation.isPending}
              >
                {createMutation.isPending ? "Creating Project..." : "Create Project"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
