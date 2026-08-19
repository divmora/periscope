import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ProjectsAPI } from "../../api/projects";
import { Button } from "@/components/ui/button";
import { Plus, Database, Cloud, ShieldAlert } from "lucide-react";
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
  source_dsn: z.string().min(5, "Database DSN is required"),
  storage_type: z.string(),
  storage_bucket: z.string().min(3, "Bucket name is required"),
  storage_region: z.string(),
  storage_endpoint: z.string().optional(),
  storage_access_key: z.string().optional(),
  storage_secret_key: z.string().optional(),
  prune_enabled: z.boolean(),
});

export function CreateProjectDialog() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      source_dsn: "",
      storage_type: "s3",
      storage_bucket: "",
      storage_region: "us-east-1",
      storage_endpoint: "",
      storage_access_key: "",
      storage_secret_key: "",
      prune_enabled: false,
    },
  });

  const createMutation = useMutation({
    mutationFn: (values: z.infer<typeof formSchema>) => ProjectsAPI.createProject(values),
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
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-2xl flex items-center gap-2">
            <Database className="h-5 w-5 text-primary" />
            Create New Project
          </DialogTitle>
          <DialogDescription>
            Connect a Laravel database to start collecting telemetry data.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
            <div className="space-y-4">
              <div className="flex items-center gap-2 pb-2 border-b">
                <Database className="h-4 w-4 text-muted-foreground" />
                <h3 className="font-semibold text-sm">General & Database</h3>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <FormField
                  control={form.control}
                  name="name"
                  render={({ field }) => (
                    <FormItem className="col-span-2 md:col-span-1">
                      <FormLabel>Project Name</FormLabel>
                      <FormControl>
                        <Input placeholder="e.g. Production API" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                
                <FormField
                  control={form.control}
                  name="source_dsn"
                  render={({ field }) => (
                    <FormItem className="col-span-2">
                      <FormLabel>Database Connection String (DSN)</FormLabel>
                      <FormControl>
                        <Input placeholder="postgres://user:pass@host:5432/db" {...field} />
                      </FormControl>
                      <FormDescription>
                        This is stored securely via AES-GCM encryption.
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="flex items-center gap-2 pb-2 border-b pt-4">
                <Cloud className="h-4 w-4 text-muted-foreground" />
                <h3 className="font-semibold text-sm">Cold Storage (S3)</h3>
              </div>
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
                <FormField
                  control={form.control}
                  name="storage_endpoint"
                  render={({ field }) => (
                    <FormItem className="col-span-2">
                      <FormLabel>Endpoint Override (MinIO, R2, etc)</FormLabel>
                      <FormControl>
                        <Input placeholder="https://..." {...field} />
                      </FormControl>
                      <FormDescription>Leave blank for standard AWS S3.</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="storage_access_key"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Access Key ID</FormLabel>
                      <FormControl>
                        <Input placeholder="Optional" {...field} />
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
                        <Input type="password" placeholder="Optional" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="flex items-center gap-2 pb-2 border-b pt-4">
                <ShieldAlert className="h-4 w-4 text-muted-foreground" />
                <h3 className="font-semibold text-sm">Data Retention</h3>
              </div>
              <FormField
                control={form.control}
                name="prune_enabled"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                    <div className="space-y-0.5">
                      <FormLabel className="text-base">Enable Auto-Pruning</FormLabel>
                      <FormDescription>
                        Automatically delete data from Laravel database after it's synced to Parquet.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Creating..." : "Create Project"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
