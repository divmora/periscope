import { Outlet, useLocation, Link, useNavigate } from "react-router-dom";
import { 
  SidebarProvider, 
  Sidebar, 
  SidebarInset, 
  SidebarTrigger, 
  SidebarHeader, 
  SidebarContent, 
  SidebarGroup, 
  SidebarMenu, 
  SidebarMenuItem, 
  SidebarMenuButton, 
  SidebarFooter, 
  SidebarGroupLabel, 
  SidebarGroupContent,
} from "@/components/ui/sidebar";
import { 
  DropdownMenu, 
  DropdownMenuContent, 
  DropdownMenuItem, 
  DropdownMenuTrigger, 
  DropdownMenuLabel, 
  DropdownMenuSeparator 
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Separator } from "@/components/ui/separator";
import { Breadcrumb, BreadcrumbList, BreadcrumbItem, BreadcrumbLink, BreadcrumbSeparator, BreadcrumbPage } from "@/components/ui/breadcrumb";
import { 
  Telescope, Database, Settings, LogOut, Activity, ArrowUpSquare, 
  Monitor, Clock, Menu, AlignLeft, FileText, Code, Headphones, Bug, Lock, 
  Globe, Archive, Mail, Layers, Megaphone, Layout, ChevronRight, Terminal, Shield
} from "lucide-react";
import { useAuth } from "../context/AuthContext";
import { ROLES } from "../lib/constants";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ThemeToggle } from "@/components/ThemeToggle";

export default function RootLayout() {
  const { user, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();

  // Extract projectId from URL if it exists and isn't a reserved route
  const pathParts = location.pathname.split('/').filter(Boolean);
  const firstPath = pathParts[0];
  const isGlobalRoute = !firstPath || firstPath === 'projects' || firstPath === 'settings';
  const projectId = !isGlobalRoute ? firstPath : null;

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const projectNavItems = projectId ? [
    { name: "Data Sources", href: `/${projectId}/sources`, icon: Layers, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "SQL Studio", href: `/${projectId}/studio`, icon: Terminal, allowedRoles: [ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Aurora DAS Audit", href: `/${projectId}/das`, icon: Shield, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Requests & Traces", href: `/${projectId}/telescope/requests`, icon: ArrowUpSquare, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Queries", href: `/${projectId}/telescope/queries`, icon: AlignLeft, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Exceptions", href: `/${projectId}/telescope/exceptions`, icon: Bug, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Logs", href: `/${projectId}/telescope/logs`, icon: Archive, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Commands", href: `/${projectId}/telescope/commands`, icon: Monitor, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Schedule", href: `/${projectId}/telescope/schedule`, icon: Clock, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Jobs", href: `/${projectId}/telescope/jobs`, icon: Menu, allowedRoles: [ROLES.VIEWER, ROLES.MEMBER, ROLES.ADMIN] },
    { name: "Project Settings", href: `/${projectId}/config`, icon: Settings, allowedRoles: [ROLES.ADMIN] },
  ] : [];


  const currentUserRole = user?.global_role || ROLES.VIEWER;
  const visibleNavItems = projectNavItems.filter(item => item.allowedRoles.includes(currentUserRole));

  return (
    <SidebarProvider>
      <Sidebar variant="inset" className="border-r border-border bg-sidebar">
        <SidebarHeader className="p-4 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <Telescope className="h-5 w-5" />
            </div>
            <div className="flex flex-col gap-0.5 leading-none">
              <span className="font-semibold tracking-tight text-sidebar-foreground">Periscope</span>
              <span className="text-[10px] uppercase text-sidebar-foreground/50 tracking-wider font-medium">Enterprise Telemetry</span>
            </div>
          </div>
        </SidebarHeader>

        <SidebarContent>
          {/* Global Navigation */}
          <SidebarGroup>
            <SidebarGroupLabel>Platform</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton asChild isActive={firstPath === 'projects'} tooltip="Projects Dashboard">
                    <Link to="/projects">
                      <Database className="h-4 w-4" />
                      <span>Projects</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                {currentUserRole === 'admin' && (
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={firstPath === 'settings'} tooltip="Global Settings">
                      <Link to="/settings">
                        <Settings className="h-4 w-4" />
                        <span>Settings</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                )}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>

          {/* Project Navigation (Only if in a project) */}
          {projectId && (
            <Collapsible defaultOpen className="group/collapsible">
              <SidebarGroup>
                <SidebarGroupLabel asChild>
                  <CollapsibleTrigger className="w-full flex items-center justify-between text-xs hover:text-sidebar-foreground transition-colors cursor-pointer">
                    Project {projectId.substring(0, 6)}...
                    <ChevronRight className="h-4 w-4 transition-transform group-data-[state=open]/collapsible:rotate-90" />
                  </CollapsibleTrigger>
                </SidebarGroupLabel>
                <CollapsibleContent>
                  <SidebarGroupContent>
                    <SidebarMenu>
                      {visibleNavItems.map((item) => {
                        const isActive = location.pathname.startsWith(item.href);
                        return (
                          <SidebarMenuItem key={item.name}>
                            <SidebarMenuButton asChild isActive={isActive} tooltip={item.name}>
                              <Link to={item.href}>
                                <item.icon className="h-4 w-4" />
                                <span>{item.name}</span>
                              </Link>
                            </SidebarMenuButton>
                          </SidebarMenuItem>
                        );
                      })}
                    </SidebarMenu>
                  </SidebarGroupContent>
                </CollapsibleContent>
              </SidebarGroup>
            </Collapsible>
          )}
        </SidebarContent>

        <SidebarFooter className="p-4 border-t border-border">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <div className="flex items-center gap-3 w-full cursor-pointer hover:bg-sidebar-accent hover:text-sidebar-accent-foreground p-2 rounded-md transition-colors">
                <Avatar className="h-8 w-8 rounded-lg border border-border">
                  <AvatarFallback className="rounded-lg bg-primary/10 text-primary text-xs font-medium">
                    {user?.username?.substring(0, 2).toUpperCase() || 'AD'}
                  </AvatarFallback>
                </Avatar>
                <div className="flex flex-col flex-1 text-left text-sm leading-tight overflow-hidden">
                  <span className="truncate font-medium text-sidebar-foreground">{user?.username || 'Administrator'}</span>
                  <span className="truncate text-xs text-sidebar-foreground/50">{user?.email || 'admin@example.com'}</span>
                </div>
              </div>
            </DropdownMenuTrigger>
            <DropdownMenuContent className="w-56" align="end" side="right" sideOffset={8}>
              <DropdownMenuLabel>My Account</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem>
                <Settings className="mr-2 h-4 w-4" />
                <span>Profile Settings</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={handleLogout} className="text-destructive focus:bg-destructive focus:text-destructive-foreground">
                <LogOut className="mr-2 h-4 w-4" />
                <span>Log out</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </SidebarFooter>
      </Sidebar>

      <SidebarInset className="bg-background flex flex-col min-w-0 flex-1 h-screen overflow-hidden">
        <header className="flex h-16 shrink-0 items-center justify-between border-b border-border bg-background px-4">
          <div className="flex items-center gap-2">
            <SidebarTrigger className="-ml-1 text-muted-foreground hover:text-foreground" />
            <Separator orientation="vertical" className="mx-2 h-4" />
            <Breadcrumb>
              <BreadcrumbList>
                <BreadcrumbItem className="hidden md:block">
                  <BreadcrumbLink asChild>
                    <Link to="/projects">Platform</Link>
                  </BreadcrumbLink>
                </BreadcrumbItem>
                <BreadcrumbSeparator className="hidden md:block" />
                {projectId ? (
                  <>
                    <BreadcrumbItem className="hidden md:block">
                      <BreadcrumbLink asChild>
                        <Link to={`/${projectId}/telescope/requests`}>Project {projectId.substring(0, 6)}</Link>
                      </BreadcrumbLink>
                    </BreadcrumbItem>
                    <BreadcrumbSeparator className="hidden md:block" />
                    <BreadcrumbItem>
                      <BreadcrumbPage className="capitalize">{pathParts[pathParts.length - 1]}</BreadcrumbPage>
                    </BreadcrumbItem>
                  </>
                ) : (
                  <BreadcrumbItem>
                    <BreadcrumbPage className="capitalize">{firstPath || 'Projects'}</BreadcrumbPage>
                  </BreadcrumbItem>
                )}
              </BreadcrumbList>
            </Breadcrumb>
          </div>
          <div className="flex items-center gap-2">
            <ThemeToggle />
          </div>
        </header>
        <main className="flex-1 overflow-auto bg-muted/20">
          <Outlet />
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}
