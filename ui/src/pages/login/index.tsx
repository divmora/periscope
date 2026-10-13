import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Telescope, Loader2, Lock } from "lucide-react";
import { useAuth } from "../../context/AuthContext";
import { AuthAPI } from "../../api/auth";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";

export default function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setIsSubmitting(true);

    try {
      const res = await AuthAPI.login(username, password);
      if (res.success && res.data) {
        login(res.data);
        navigate("/projects");
      } else {
        setError(res.error?.message || "Login failed");
      }
    } catch (err: any) {
      setError(err.message || "An unexpected error occurred");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="grid min-h-screen lg:grid-cols-2 bg-background">
      <div className="flex flex-col gap-4 p-6 md:p-10">
        <div className="flex justify-center gap-2 md:justify-start">
          <a href="#" className="flex items-center gap-2 font-bold text-xl">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <Telescope className="size-4" />
            </div>
            Periscope
          </a>
        </div>
        <div className="flex flex-1 items-center justify-center">
          <div className="w-full max-w-sm">
            <div className="flex flex-col space-y-2 text-center mb-8">
              <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                Welcome back
              </h1>
              <p className="text-sm text-muted-foreground">
                Enter your credentials to access your dashboard
              </p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-sm text-center">
                  {error}
                </div>
              )}

              <div className="space-y-2 text-left">
                <Label htmlFor="username">Username or Email</Label>
                <Input
                  id="username"
                  type="text"
                  autoComplete="username"
                  value={username}
                  onChange={(e: any) => setUsername(e.target.value)}
                  placeholder="admin@example.com"
                  required
                  className="bg-muted/50 border-border focus-visible:ring-primary h-11"
                />
              </div>

              <div className="space-y-2 text-left">
                <div className="flex items-center justify-between">
                  <Label htmlFor="password">Password</Label>
                  <a href="#" className="text-xs text-primary hover:underline">Forgot password?</a>
                </div>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e: any) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  required
                  className="bg-muted/50 border-border focus-visible:ring-primary h-11"
                />
              </div>

              <Button
                type="submit"
                className="w-full h-11 mt-6 bg-primary text-primary-foreground hover:bg-primary/90 shadow-md"
                disabled={isSubmitting}
              >
                {isSubmitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Lock className="mr-2 h-4 w-4" />}
                Sign In
              </Button>
            </form>

            <div className="mt-8 text-center text-xs text-muted-foreground">
              Built with love by DivMora.
            </div>
          </div>
        </div>
      </div>
      <div className="relative hidden lg:block overflow-hidden bg-zinc-950 border-l border-border">
        {/* Abstract Dark Background */}
        <div className="absolute inset-0 bg-gradient-to-tr from-indigo-950 via-zinc-900 to-zinc-950" />

        {/* Decorative Glowing Orbs */}
        <div className="absolute -top-24 -right-24 h-96 w-96 rounded-full bg-primary/20 blur-[120px]" />
        <div className="absolute bottom-12 -left-12 h-64 w-64 rounded-full bg-blue-600/10 blur-[100px]" />

        {/* Overlay Content */}
        <div className="absolute inset-0 flex flex-col items-center justify-center p-12 text-center z-10">
          <div className="h-24 w-24 rounded-3xl bg-white/10 border border-white/20 shadow-2xl flex items-center justify-center backdrop-blur-md mb-8">
            <Telescope className="h-12 w-12 text-white drop-shadow-[0_0_15px_rgba(255,255,255,0.5)]" />
          </div>
          <h2 className="text-3xl font-bold text-white mb-4 tracking-tight">Enterprise Telemetry Lakehouse</h2>
          <p className="text-base text-zinc-400 max-w-lg mb-8 leading-relaxed">
            High-performance, Parquet-backed cold storage &amp; DuckDB analytical engine for application telemetry, distributed traces, database activity streams, and system audit logs.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-2 max-w-md">
            <span className="px-3 py-1 rounded-full text-xs font-medium bg-white/5 border border-white/10 text-zinc-300">
              Laravel Telescope
            </span>
            <span className="px-3 py-1 rounded-full text-xs font-medium bg-white/5 border border-white/10 text-zinc-300">
              AWS Aurora DAS
            </span>
            <span className="px-3 py-1 rounded-full text-xs font-medium bg-white/5 border border-white/10 text-zinc-300">
              PostgreSQL Audit
            </span>
            <span className="px-3 py-1 rounded-full text-xs font-medium bg-white/5 border border-white/10 text-zinc-300">
              DuckDB OLAP
            </span>
            <span className="px-3 py-1 rounded-full text-xs font-medium bg-white/5 border border-white/10 text-zinc-300">
              OpenTelemetry Ready
            </span>
          </div>
        </div>

        {/* Subtle Grid Pattern overlay */}
        <div className="absolute inset-0 bg-[url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCI+PGRlZnM+PHBhdHRlcm4gaWQ9ImdyaWQiIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgcGF0dGVyblVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+PHBhdGggZD0iTSA0MCAwIEwgMCAwIDAgNDAiIGZpbGw9Im5vbmUiIHN0cm9rZT0icmdiYSgyNTUsIDI1NSwgMjU1LCAwLjAyKSIgc3Ryb2tlLXdpZHRoPSIxIi8+PC9wYXR0ZXJuPjwvZGVmcz48cmVjdCB3aWR0aD0iMTAwJSIgaGVpZ2h0PSIxMDAlIiBmaWxsPSJ1cmwoI2dyaWQpIi8+PC9zdmc+')] mix-blend-overlay opacity-50" />
      </div>
    </div>
  );
}
