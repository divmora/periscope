import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { TeamsAPI, ProjectTeam } from "../../api/teams";
import { ProjectSource } from "../../api/sources";
import { 
  Users, Plus, Trash2, UserPlus, 
  Database, Loader2, Check
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

interface TeamsManagerProps {
  projectId: string;
  sources: ProjectSource[];
}

export function TeamsManager({ projectId, sources }: TeamsManagerProps) {
  const queryClient = useQueryClient();

  // Dialog States
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [membersDialogOpen, setMembersDialogOpen] = useState(false);
  const [selectedTeam, setSelectedTeam] = useState<ProjectTeam | null>(null);

  // Create Team Form State
  const [teamName, setTeamName] = useState("");
  const [teamDescription, setTeamDescription] = useState("");
  const [teamRole, setTeamRole] = useState<"viewer" | "member" | "admin">("viewer");
  const [unrestrictedSources, setUnrestrictedSources] = useState(true);
  const [selectedSources, setSelectedSources] = useState<string[]>([]);
  const [formError, setFormError] = useState("");

  // Add Member to Team Form State
  const [memberIdentifier, setMemberIdentifier] = useState("");
  const [memberError, setMemberError] = useState("");

  // Queries
  const { data: teamsData, isLoading } = useQuery({
    queryKey: ['project_teams', projectId],
    queryFn: () => TeamsAPI.listTeams(projectId),
    enabled: !!projectId,
  });

  const { data: teamMembersData, isLoading: isLoadingMembers } = useQuery({
    queryKey: ['team_members', selectedTeam?.iid],
    queryFn: () => selectedTeam ? TeamsAPI.listTeamMembers(projectId, selectedTeam.iid) : null,
    enabled: !!selectedTeam && membersDialogOpen,
  });

  const teams = teamsData?.data || [];
  const teamMembers = teamMembersData?.data || [];

  // Mutations
  const createMutation = useMutation({
    mutationFn: () => {
      const allowed = unrestrictedSources ? [] : selectedSources;
      return TeamsAPI.createTeam(projectId, {
        name: teamName.trim(),
        description: teamDescription.trim(),
        role: teamRole,
        allowed_sources: allowed.length > 0 ? allowed : undefined,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project_teams', projectId] });
      setCreateDialogOpen(false);
      resetCreateForm();
    },
    onError: (err: any) => {
      setFormError(err?.message || "Failed to create team");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (teamIid: string) => TeamsAPI.deleteTeam(projectId, teamIid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project_teams', projectId] });
    },
  });

  const addMemberMutation = useMutation({
    mutationFn: () => {
      if (!selectedTeam) throw new Error("No team selected");
      return TeamsAPI.addTeamMember(projectId, selectedTeam.iid, memberIdentifier.trim());
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team_members', selectedTeam?.iid] });
      queryClient.invalidateQueries({ queryKey: ['project_teams', projectId] });
      setMemberIdentifier("");
      setMemberError("");
    },
    onError: (err: any) => {
      setMemberError(err?.message || "Failed to add member to team");
    },
  });

  const removeMemberMutation = useMutation({
    mutationFn: (userIid: string) => {
      if (!selectedTeam) throw new Error("No team selected");
      return TeamsAPI.removeTeamMember(projectId, selectedTeam.iid, userIid);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team_members', selectedTeam?.iid] });
      queryClient.invalidateQueries({ queryKey: ['project_teams', projectId] });
    },
  });

  const resetCreateForm = () => {
    setTeamName("");
    setTeamDescription("");
    setTeamRole("viewer");
    setUnrestrictedSources(true);
    setSelectedSources([]);
    setFormError("");
  };

  const toggleSourceSelection = (sourceIid: string) => {
    if (selectedSources.includes(sourceIid)) {
      setSelectedSources(selectedSources.filter((id) => id !== sourceIid));
    } else {
      setSelectedSources([...selectedSources, sourceIid]);
    }
  };

  const getSourceDisplayName = (sourceIid: string) => {
    const s = sources.find((src) => src.iid === sourceIid);
    return s ? s.name : sourceIid.slice(0, 8);
  };

  return (
    <div className="space-y-6">
      {/* Header & Create Team Button */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h3 className="text-sm font-semibold text-foreground">Project Teams</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Organize users into teams with granular data source access and roles (Grafana / Datadog RBAC model).
          </p>
        </div>
        <Button 
          onClick={() => {
            resetCreateForm();
            setCreateDialogOpen(true);
          }}
          className="gap-2 bg-primary text-primary-foreground hover:bg-primary/90 shadow-sm"
          size="sm"
        >
          <Plus className="h-4 w-4" />
          Create Team
        </Button>
      </div>

      {/* Teams Grid / List */}
      {isLoading ? (
        <div className="flex items-center justify-center p-8 border border-border rounded-lg bg-card">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : teams.length === 0 ? (
        <div className="border border-dashed border-border rounded-lg p-8 text-center bg-card/40">
          <Users className="h-8 w-8 text-muted-foreground mx-auto mb-2 opacity-50" />
          <h4 className="text-sm font-medium text-foreground">No Teams Created Yet</h4>
          <p className="text-xs text-muted-foreground max-w-sm mx-auto mt-1 mb-4">
            Teams let you group members and restrict access to specific telemetry sources (like isolating Telescope vs. Aurora DAS).
          </p>
          <Button 
            onClick={() => setCreateDialogOpen(true)}
            variant="outline" 
            size="sm"
            className="gap-2"
          >
            <Plus className="h-4 w-4" />
            Create First Team
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {teams.map((team) => (
            <div 
              key={team.iid} 
              className="bg-card border border-border rounded-xl p-5 shadow-sm hover:border-border/80 transition-all flex flex-col justify-between"
            >
              <div>
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <h4 className="font-semibold text-sm text-foreground flex items-center gap-2">
                      <Users className="h-4 w-4 text-primary" />
                      {team.name}
                    </h4>
                    {team.description && (
                      <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
                        {team.description}
                      </p>
                    )}
                  </div>
                  <span className={`text-[10px] font-semibold px-2 py-0.5 rounded uppercase tracking-wider ${
                    team.role === "admin" 
                      ? "bg-purple-500/10 text-purple-400 border border-purple-500/20"
                      : team.role === "member"
                      ? "bg-blue-500/10 text-blue-400 border border-blue-500/20"
                      : "bg-zinc-500/10 text-zinc-400 border border-zinc-500/20"
                  }`}>
                    {team.role}
                  </span>
                </div>

                {/* Allowed Data Sources */}
                <div className="mt-4 pt-3 border-t border-border">
                  <span className="text-[11px] font-medium text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                    <Database className="h-3 w-3 text-muted-foreground" />
                    Accessible Sources:
                  </span>
                  <div className="flex flex-wrap gap-1.5">
                    {!team.allowed_sources || team.allowed_sources.length === 0 ? (
                      <span className="text-[10px] font-medium px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1">
                        <Check className="h-3 w-3" />
                        All Project Sources (Unrestricted)
                      </span>
                    ) : (
                      team.allowed_sources.map((srcIid) => (
                        <span 
                          key={srcIid}
                          className="text-[10px] font-medium px-2 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20"
                        >
                          {getSourceDisplayName(srcIid)}
                        </span>
                      ))
                    )}
                  </div>
                </div>
              </div>

              {/* Footer: Member Count & Actions */}
              <div className="mt-5 pt-3 border-t border-border flex items-center justify-between">
                <span className="text-xs text-muted-foreground">
                  <strong className="text-foreground">{team.member_count}</strong> {team.member_count === 1 ? "member" : "members"}
                </span>

                <div className="flex items-center gap-2">
                  <Button 
                    variant="outline" 
                    size="sm" 
                    className="h-8 text-xs gap-1.5"
                    onClick={() => {
                      setSelectedTeam(team);
                      setMembersDialogOpen(true);
                    }}
                  >
                    <Users className="h-3.5 w-3.5" />
                    Members
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 w-8 p-0 text-muted-foreground hover:text-destructive"
                    onClick={() => {
                      if (window.confirm(`Are you sure you want to delete team "${team.name}"?`)) {
                        deleteMutation.mutate(team.iid);
                      }
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Dialog: Create Team */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent className="max-w-md bg-card border-border">
          <DialogHeader>
            <DialogTitle className="text-lg flex items-center gap-2">
              <Users className="h-5 w-5 text-primary" />
              Create Project Team
            </DialogTitle>
            <DialogDescription className="text-xs text-muted-foreground">
              Define a team, specify its role, and restrict access to designated data sources.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Team Name</label>
              <Input 
                value={teamName}
                onChange={(e) => setTeamName(e.target.value)}
                placeholder="e.g. SRE &amp; Security, Backend Devs"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Description (Optional)</label>
              <Input 
                value={teamDescription}
                onChange={(e) => setTeamDescription(e.target.value)}
                placeholder="e.g. Access limited to production audit streams"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Team Role</label>
              <select
                value={teamRole}
                onChange={(e) => setTeamRole(e.target.value as "viewer" | "member" | "admin")}
                className="w-full bg-muted/40 border border-border rounded-md px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary"
              >
                <option value="viewer">Viewer (Read-only access)</option>
                <option value="member">Member (Sync &amp; SQL Query)</option>
                <option value="admin">Admin (Full Control)</option>
              </select>
            </div>

            {/* Source Access Permissions */}
            <div className="space-y-2 pt-2 border-t border-border">
              <div className="flex items-center justify-between">
                <label className="text-xs font-medium text-foreground">Data Source Access</label>
                <button
                  type="button"
                  onClick={() => setUnrestrictedSources(!unrestrictedSources)}
                  className={`text-xs font-medium cursor-pointer ${
                    unrestrictedSources ? "text-emerald-400" : "text-primary"
                  }`}
                >
                  {unrestrictedSources ? "All Sources (Default)" : "Custom Sources Selection"}
                </button>
              </div>

              {unrestrictedSources ? (
                <div className="p-3 rounded-lg border border-border bg-muted/20 text-xs text-muted-foreground flex items-center gap-2">
                  <Check className="h-4 w-4 text-emerald-400 shrink-0" />
                  <span>Team members will have access to <strong>all current and future data sources</strong> in this project.</span>
                </div>
              ) : (
                <div className="space-y-2 max-h-48 overflow-y-auto pr-1">
                  {sources.length === 0 ? (
                    <p className="text-xs text-muted-foreground py-2 text-center">
                      No sources configured in this project yet.
                    </p>
                  ) : (
                    sources.map((src) => {
                      const isChecked = selectedSources.includes(src.iid);
                      return (
                        <div
                          key={src.iid}
                          onClick={() => toggleSourceSelection(src.iid)}
                          className={`flex items-center justify-between p-2.5 rounded-lg border text-xs cursor-pointer transition-all ${
                            isChecked
                              ? "border-primary bg-primary/10 text-foreground"
                              : "border-border bg-muted/20 text-muted-foreground hover:bg-muted/40"
                          }`}
                        >
                          <div className="flex items-center gap-2">
                            <Database className={`h-3.5 w-3.5 ${isChecked ? "text-primary" : "text-muted-foreground"}`} />
                            <span className="font-medium">{src.name}</span>
                          </div>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-muted/60 text-muted-foreground uppercase font-mono">
                            {src.source_type}
                          </span>
                        </div>
                      );
                    })
                  )}
                </div>
              )}
            </div>

            {formError && <p className="text-xs text-destructive font-medium">{formError}</p>}
          </div>

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setCreateDialogOpen(false)}>
              Cancel
            </Button>
            <Button 
              size="sm"
              disabled={createMutation.isPending || !teamName.trim()}
              onClick={() => createMutation.mutate()}
            >
              {createMutation.isPending ? "Creating..." : "Create Team"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Dialog: Manage Team Members */}
      <Dialog open={membersDialogOpen} onOpenChange={setMembersDialogOpen}>
        <DialogContent className="max-w-lg bg-card border-border">
          <DialogHeader>
            <DialogTitle className="text-lg flex items-center gap-2">
              <Users className="h-5 w-5 text-primary" />
              Manage Team Members: {selectedTeam?.name}
            </DialogTitle>
            <DialogDescription className="text-xs text-muted-foreground">
              Add or remove project users from this team.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            {/* Add Member Form */}
            <form 
              onSubmit={(e) => {
                e.preventDefault();
                if (memberIdentifier.trim()) addMemberMutation.mutate();
              }}
              className="flex gap-2"
            >
              <Input
                placeholder="User email, username, or UUID"
                value={memberIdentifier}
                onChange={(e) => setMemberIdentifier(e.target.value)}
                className="text-xs"
              />
              <Button 
                type="submit" 
                size="sm"
                disabled={addMemberMutation.isPending || !memberIdentifier.trim()}
                className="gap-1.5 shrink-0"
              >
                <UserPlus className="h-3.5 w-3.5" />
                Add
              </Button>
            </form>

            {memberError && <p className="text-xs text-destructive font-medium">{memberError}</p>}

            {/* Team Members List */}
            <div className="border border-border rounded-lg overflow-hidden max-h-60 overflow-y-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-muted/50 text-muted-foreground uppercase border-b border-border">
                  <tr>
                    <th className="px-3 py-2">Member</th>
                    <th className="px-3 py-2">Added</th>
                    <th className="px-3 py-2 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {isLoadingMembers ? (
                    <tr>
                      <td colSpan={3} className="px-3 py-6 text-center text-muted-foreground">
                        <Loader2 className="h-4 w-4 animate-spin mx-auto text-primary" />
                      </td>
                    </tr>
                  ) : teamMembers.length === 0 ? (
                    <tr>
                      <td colSpan={3} className="px-3 py-6 text-center text-muted-foreground">
                        No users assigned to this team yet.
                      </td>
                    </tr>
                  ) : (
                    teamMembers.map((m) => (
                      <tr key={m.user_iid} className="hover:bg-muted/20">
                        <td className="px-3 py-2">
                          <div className="font-medium text-foreground">{m.username}</div>
                          <div className="text-[11px] text-muted-foreground">{m.email}</div>
                        </td>
                        <td className="px-3 py-2 text-muted-foreground">
                          {new Date(m.created_at).toLocaleDateString()}
                        </td>
                        <td className="px-3 py-2 text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                            onClick={() => removeMemberMutation.mutate(m.user_iid)}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setMembersDialogOpen(false)}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
