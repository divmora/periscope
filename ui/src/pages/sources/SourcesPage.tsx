import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { SourcesAPI, ProjectSource } from "../../api/sources";
import { 
  Layers, Plus, Trash2, ShieldCheck, Database, 
  Radio, CheckCircle2, Clock, Loader2, Sparkles
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function SourcesPage() {
  const { projectId } = useParams();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);

  // Form State
  const [name, setName] = useState("");
  const [sourceType, setSourceType] = useState<ProjectSource["source_type"]>("laravel_telescope");
  const [dsn, setDsn] = useState("");
  const [dasEngine, setDasEngine] = useState<"direct" | "athena">("direct");
  const [s3Bucket, setS3Bucket] = useState("");
  const [s3Prefix, setS3Prefix] = useState("aurora_das/");
  const [s3Region, setS3Region] = useState("us-east-1");
  const [athenaDatabase, setAthenaDatabase] = useState("");
  const [athenaTable, setAthenaTable] = useState("activity_stream");
  const [athenaWorkgroup, setAthenaWorkgroup] = useState("primary");
  const [athenaOutput, setAthenaOutput] = useState("");

  // AWS Authentication State
  const [awsAuthMode, setAwsAuthMode] = useState<"default" | "profile" | "keys" | "role">("default");
  const [awsProfile, setAwsProfile] = useState("");
  const [awsAccessKeyId, setAwsAccessKeyId] = useState("");
  const [awsSecretAccessKey, setAwsSecretAccessKey] = useState("");
  const [awsSessionToken, setAwsSessionToken] = useState("");
  const [awsRoleArn, setAwsRoleArn] = useState("");
  const [awsExternalId, setAwsExternalId] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ['project_sources', projectId],
    queryFn: () => SourcesAPI.listSources(projectId as string),
    enabled: !!projectId,
  });

  const sources = data?.data || [];

  const createMutation = useMutation({
    mutationFn: () => {
      const config: Record<string, any> = {};
      if (sourceType === "aurora_das") {
        config.engine = dasEngine;
        config.aws_auth_mode = awsAuthMode;
        if (awsAuthMode === "profile") {
          config.aws_profile = awsProfile;
        } else if (awsAuthMode === "keys") {
          config.aws_access_key_id = awsAccessKeyId;
          config.aws_secret_access_key = awsSecretAccessKey;
          if (awsSessionToken) config.aws_session_token = awsSessionToken;
        } else if (awsAuthMode === "role") {
          config.aws_role_arn = awsRoleArn;
          if (awsExternalId) config.aws_external_id = awsExternalId;
        }

        if (dasEngine === "direct") {
          config.s3_bucket = s3Bucket;
          config.s3_prefix = s3Prefix;
          config.s3_region = s3Region;
        } else {
          config.athena_database = athenaDatabase;
          config.athena_table = athenaTable;
          config.athena_workgroup = athenaWorkgroup;
          config.athena_output_location = athenaOutput;
          config.aws_region = s3Region;
        }
      }
      return SourcesAPI.createSource(projectId as string, {
        name,
        source_type: sourceType,
        dsn: sourceType === "aurora_das" ? (dasEngine === "direct" ? `s3://${s3Bucket}/${s3Prefix}` : `athena://${athenaDatabase}/${athenaTable}`) : dsn,
        status: "active",
        config,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project_sources', projectId] });
      setOpen(false);
      setName("");
      setDsn("");
      setS3Bucket("");
      setAthenaDatabase("");
      setAthenaOutput("");
      setAwsAuthMode("default");
      setAwsProfile("");
      setAwsAccessKeyId("");
      setAwsSecretAccessKey("");
      setAwsSessionToken("");
      setAwsRoleArn("");
      setAwsExternalId("");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (sourceIid: string) => SourcesAPI.deleteSource(projectId as string, sourceIid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project_sources', projectId] });
    },
  });

  const getSourceBadge = (type: string) => {
    switch (type) {
      case "laravel_telescope":
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-500/10 text-red-400 border border-red-500/20">Laravel Telescope</span>;
      case "aurora_das":
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">AWS Aurora DAS</span>;
      case "postgres_audit":
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">PostgreSQL Audit</span>;
      case "opentelemetry":
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">OpenTelemetry</span>;
      default:
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-medium bg-zinc-500/10 text-zinc-400 border border-zinc-500/20">{type}</span>;
    }
  };

  return (
    <div className="flex-1 space-y-6 p-8 pt-6">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-3">
              Data Ingestion Sources
              <span className="px-2.5 py-0.5 rounded-full bg-muted border border-border text-xs font-medium text-muted-foreground">
                {sources.length} active
              </span>
            </h1>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            Connect and orchestrate multi-dialect database telemetry pipelines into Hive-partitioned Parquet storage.
          </p>
        </div>

        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button className="bg-primary text-primary-foreground hover:bg-primary/90">
              <Plus className="mr-2 h-4 w-4" />
              Add Ingestion Source
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto bg-card border-border">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Layers className="h-5 w-5 text-primary" />
                Add Data Source
              </DialogTitle>
              <DialogDescription>
                Attach a new database stream or telemetry collector to this project.
              </DialogDescription>
            </DialogHeader>

            <form
              onSubmit={(e) => {
                e.preventDefault();
                createMutation.mutate();
              }}
              className="space-y-4"
            >
              <div className="space-y-2">
                <Label htmlFor="source-name">Source Name</Label>
                <Input
                  id="source-name"
                  placeholder="e.g. Staging MySQL or Main Aurora Cluster"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="source-type">Source Protocol / Engine</Label>
                <select
                  id="source-type"
                  value={sourceType}
                  onChange={(e) => setSourceType(e.target.value as any)}
                  className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary"
                >
                  <option value="laravel_telescope">Laravel Telescope (MySQL / Postgres)</option>
                  <option value="aurora_das">AWS Aurora Database Activity Streams (DAS)</option>
                  <option value="postgres_audit">PostgreSQL Audit Logs (pgaudit / audit tables)</option>
                  <option value="opentelemetry">OpenTelemetry (OTLP gRPC/HTTP Collector)</option>
                  <option value="generic_db">Generic Database Log Table</option>
                </select>
              </div>

              {sourceType === "aurora_das" ? (
                <div className="space-y-4 pt-2 border-t border-border/60">
                  <div className="space-y-2">
                    <Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">DAS Query Engine</Label>
                    <div className="grid grid-cols-2 gap-2">
                      <button
                        type="button"
                        onClick={() => setDasEngine("direct")}
                        className={`p-3 rounded-lg border text-left transition-all flex flex-col justify-between ${
                          dasEngine === "direct"
                            ? "border-primary bg-primary/10 text-foreground"
                            : "border-border bg-card hover:bg-muted/30 text-muted-foreground"
                        }`}
                      >
                        <div className="flex items-center justify-between mb-1">
                          <span className="font-semibold text-xs flex items-center gap-1.5">
                            <Sparkles className="h-3.5 w-3.5 text-amber-400" />
                            Direct S3
                          </span>
                          {dasEngine === "direct" && <CheckCircle2 className="h-3.5 w-3.5 text-primary" />}
                        </div>
                        <span className="text-[11px] text-muted-foreground leading-tight">
                          Embedded DuckDB reading Hive Parquet. Zero query cost.
                        </span>
                      </button>

                      <button
                        type="button"
                        onClick={() => setDasEngine("athena")}
                        className={`p-3 rounded-lg border text-left transition-all flex flex-col justify-between ${
                          dasEngine === "athena"
                            ? "border-primary bg-primary/10 text-foreground"
                            : "border-border bg-card hover:bg-muted/30 text-muted-foreground"
                        }`}
                      >
                        <div className="flex items-center justify-between mb-1">
                          <span className="font-semibold text-xs flex items-center gap-1.5">
                            <Radio className="h-3.5 w-3.5 text-sky-400" />
                            AWS Athena
                          </span>
                          {dasEngine === "athena" && <CheckCircle2 className="h-3.5 w-3.5 text-primary" />}
                        </div>
                        <span className="text-[11px] text-muted-foreground leading-tight">
                          Serverless query engine for petabyte-scale history.
                        </span>
                      </button>
                    </div>
                  </div>

                  {dasEngine === "direct" ? (
                    <div className="space-y-3 p-3 rounded-lg bg-muted/20 border border-border/60">
                      <div className="space-y-1.5">
                        <Label htmlFor="s3-bucket" className="text-xs">S3 Bucket Name</Label>
                        <Input
                          id="s3-bucket"
                          placeholder="e.g. my-company-aurora-das-parquet"
                          value={s3Bucket}
                          onChange={(e) => setS3Bucket(e.target.value)}
                          required
                        />
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <div className="space-y-1.5">
                          <Label htmlFor="s3-prefix" className="text-xs">S3 Prefix</Label>
                          <Input
                            id="s3-prefix"
                            placeholder="aurora_das/"
                            value={s3Prefix}
                            onChange={(e) => setS3Prefix(e.target.value)}
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label htmlFor="s3-region" className="text-xs">AWS Region</Label>
                          <Input
                            id="s3-region"
                            placeholder="us-east-1"
                            value={s3Region}
                            onChange={(e) => setS3Region(e.target.value)}
                          />
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-3 p-3 rounded-lg bg-muted/20 border border-border/60">
                      <div className="grid grid-cols-2 gap-2">
                        <div className="space-y-1.5">
                          <Label htmlFor="athena-db" className="text-xs">Glue Database</Label>
                          <Input
                            id="athena-db"
                            placeholder="e.g. audit_db"
                            value={athenaDatabase}
                            onChange={(e) => setAthenaDatabase(e.target.value)}
                            required
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label htmlFor="athena-tbl" className="text-xs">Glue Table</Label>
                          <Input
                            id="athena-tbl"
                            placeholder="e.g. aurora_das_logs"
                            value={athenaTable}
                            onChange={(e) => setAthenaTable(e.target.value)}
                            required
                          />
                        </div>
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor="athena-wg" className="text-xs">Athena Workgroup</Label>
                        <Input
                          id="athena-wg"
                          placeholder="primary"
                          value={athenaWorkgroup}
                          onChange={(e) => setAthenaWorkgroup(e.target.value)}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor="athena-out" className="text-xs">Query Result S3 Location</Label>
                        <Input
                          id="athena-out"
                          placeholder="s3://my-athena-results-bucket/queries/"
                          value={athenaOutput}
                          onChange={(e) => setAthenaOutput(e.target.value)}
                          required
                        />
                      </div>
                    </div>
                  )}

                  {/* AWS Authentication Method Selector */}
                  <div className="space-y-3 pt-3 border-t border-border/60">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        AWS Authentication
                      </Label>
                      <span className="text-[11px] text-muted-foreground font-medium">
                        {awsAuthMode === "default" && "Host / IRSA Default"}
                        {awsAuthMode === "profile" && "Named Profile (~/.aws)"}
                        {awsAuthMode === "keys" && "Static Key Pair"}
                        {awsAuthMode === "role" && "STS AssumeRole"}
                      </span>
                    </div>

                    <div className="grid grid-cols-4 gap-1.5 p-1 bg-muted/40 rounded-lg border border-border">
                      <button
                        type="button"
                        onClick={() => setAwsAuthMode("default")}
                        className={`px-2 py-1.5 rounded text-xs font-medium transition-all ${
                          awsAuthMode === "default"
                            ? "bg-primary text-primary-foreground shadow-sm"
                            : "text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        Default
                      </button>
                      <button
                        type="button"
                        onClick={() => setAwsAuthMode("profile")}
                        className={`px-2 py-1.5 rounded text-xs font-medium transition-all ${
                          awsAuthMode === "profile"
                            ? "bg-primary text-primary-foreground shadow-sm"
                            : "text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        Profile
                      </button>
                      <button
                        type="button"
                        onClick={() => setAwsAuthMode("keys")}
                        className={`px-2 py-1.5 rounded text-xs font-medium transition-all ${
                          awsAuthMode === "keys"
                            ? "bg-primary text-primary-foreground shadow-sm"
                            : "text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        Keys
                      </button>
                      <button
                        type="button"
                        onClick={() => setAwsAuthMode("role")}
                        className={`px-2 py-1.5 rounded text-xs font-medium transition-all ${
                          awsAuthMode === "role"
                            ? "bg-primary text-primary-foreground shadow-sm"
                            : "text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        IAM Role
                      </button>
                    </div>

                    {awsAuthMode === "default" && (
                      <p className="text-xs text-muted-foreground bg-muted/20 p-2.5 rounded border border-border/40 leading-relaxed">
                        Uses standard AWS credential chain (EKS IRSA, ECS Task Role, EC2 IMDSv2, or container environment variables).
                      </p>
                    )}

                    {awsAuthMode === "profile" && (
                      <div className="space-y-1.5 p-3 rounded-lg bg-muted/20 border border-border/60">
                        <Label htmlFor="aws-profile" className="text-xs">Named AWS Profile</Label>
                        <Input
                          id="aws-profile"
                          placeholder="e.g. default, staging, or production-audit"
                          value={awsProfile}
                          onChange={(e) => setAwsProfile(e.target.value)}
                          required
                        />
                        <p className="text-[11px] text-muted-foreground">
                          Loaded from standard ~/.aws/credentials or ~/.aws/config on the host.
                        </p>
                      </div>
                    )}

                    {awsAuthMode === "keys" && (
                      <div className="space-y-2.5 p-3 rounded-lg bg-muted/20 border border-border/60">
                        <div className="space-y-1.5">
                          <Label htmlFor="aws-key" className="text-xs">AWS Access Key ID</Label>
                          <Input
                            id="aws-key"
                            placeholder="AKIA..."
                            value={awsAccessKeyId}
                            onChange={(e) => setAwsAccessKeyId(e.target.value)}
                            required
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label htmlFor="aws-secret" className="text-xs">AWS Secret Access Key</Label>
                          <Input
                            id="aws-secret"
                            type="password"
                            placeholder="Secret access key"
                            value={awsSecretAccessKey}
                            onChange={(e) => setAwsSecretAccessKey(e.target.value)}
                            required
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label htmlFor="aws-token" className="text-xs">Session Token (Optional)</Label>
                          <Input
                            id="aws-token"
                            type="password"
                            placeholder="Temporary STS token (optional)"
                            value={awsSessionToken}
                            onChange={(e) => setAwsSessionToken(e.target.value)}
                          />
                        </div>
                      </div>
                    )}

                    {awsAuthMode === "role" && (
                      <div className="space-y-2.5 p-3 rounded-lg bg-muted/20 border border-border/60">
                        <div className="space-y-1.5">
                          <Label htmlFor="aws-role" className="text-xs">IAM Role ARN to Assume</Label>
                          <Input
                            id="aws-role"
                            placeholder="arn:aws:iam::123456789012:role/DasAuditReader"
                            value={awsRoleArn}
                            onChange={(e) => setAwsRoleArn(e.target.value)}
                            required
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label htmlFor="aws-ext-id" className="text-xs">External ID (Optional)</Label>
                          <Input
                            id="aws-ext-id"
                            placeholder="STS External ID for cross-account security"
                            value={awsExternalId}
                            onChange={(e) => setAwsExternalId(e.target.value)}
                          />
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              ) : (
                sourceType !== "opentelemetry" && (
                  <div className="space-y-2">
                    <Label htmlFor="source-dsn">Connection String / DSN</Label>
                    <Input
                      id="source-dsn"
                      type="password"
                      placeholder="postgres://user:pass@host:5432/db"
                      value={dsn}
                      onChange={(e) => setDsn(e.target.value)}
                      required
                    />
                    <p className="text-xs text-muted-foreground flex items-center gap-1">
                      <ShieldCheck className="h-3.5 w-3.5 text-emerald-500" />
                      Encrypted with AES-GCM before saving to metadata catalog.
                    </p>
                  </div>
                )
              )}

              <DialogFooter className="pt-2">
                <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                  Cancel
                </Button>
                <Button type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending ? "Adding..." : "Add Source"}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {/* Sources Grid */}
      {isLoading ? (
        <div className="flex items-center justify-center p-12">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
        </div>
      ) : sources.length === 0 ? (
        <div className="border border-dashed border-border rounded-xl p-12 text-center bg-card/40">
          <Layers className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-foreground mb-2">No Ingestion Sources Configured</h3>
          <p className="text-sm text-muted-foreground max-w-md mx-auto mb-6">
            Attach a source database (Laravel Telescope, Aurora DAS, or PostgreSQL Audit) to start continuously draining telemetry into cold storage.
          </p>
          <Button onClick={() => setOpen(true)} className="bg-primary text-primary-foreground">
            <Plus className="mr-2 h-4 w-4" />
            Connect First Source
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {sources.map((s) => (
            <div
              key={s.iid}
              className="rounded-xl border border-border bg-card p-5 shadow-sm hover:border-border/80 transition-all flex flex-col justify-between"
            >
              <div>
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2.5">
                    <div className="p-2 rounded-lg bg-primary/10 text-primary">
                      <Database className="h-4 w-4" />
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground text-sm leading-tight">{s.name}</h4>
                      <p className="text-[11px] text-muted-foreground font-mono mt-0.5">
                        {s.iid.substring(0, 13)}...
                      </p>
                    </div>
                  </div>
                  <span className="flex items-center gap-1 text-[11px] font-medium text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full border border-emerald-500/20">
                    <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                    Active
                  </span>
                </div>

                <div className="space-y-2 mt-4 text-xs">
                  <div className="flex items-center justify-between py-1 border-b border-border/50">
                    <span className="text-muted-foreground">Type</span>
                    {getSourceBadge(s.source_type)}
                  </div>
                  <div className="flex items-center justify-between py-1 border-b border-border/50">
                    <span className="text-muted-foreground">Connected</span>
                    <span className="text-foreground">{new Date(s.created_at).toLocaleDateString()}</span>
                  </div>
                </div>
              </div>

              <div className="mt-5 pt-3 border-t border-border/60 flex items-center justify-between">
                <span className="text-[11px] text-muted-foreground flex items-center gap-1">
                  <Clock className="h-3 w-3 text-primary" />
                  {s.source_type === "aurora_das" ? "External Lakehouse" : "Auto-syncing"}
                </span>
                <button
                  onClick={() => {
                    if (window.confirm(`Delete data source "${s.name}"?`)) {
                      deleteMutation.mutate(s.iid);
                    }
                  }}
                  className="text-muted-foreground hover:text-destructive p-1 rounded transition-colors"
                  title="Delete source"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
