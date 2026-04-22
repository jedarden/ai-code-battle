// Simple hash-based router for single-page navigation
// §16.14: integrated with back-cache for instant back/forward navigation

export type RouteHandler = (params: Record<string, string>) => void | Promise<void>;

interface Route {
  pattern: RegExp;
  handler: RouteHandler;
  paramNames: string[];
}

class Router {
  private routes: Route[] = [];
  private notFoundHandler: RouteHandler | null = null;
  private beforeNavigateHooks: Array<(from: string, to: string) => void> = [];
  private afterNavigateHooks: Array<(path: string) => void> = [];

  /**
   * Register a route with a pattern like "/leaderboard" or "/bot/:id"
   */
  on(pattern: string, handler: RouteHandler): this {
    const paramNames: string[] = [];
    const regexPattern = pattern.replace(/:(\w+)/g, (_, name) => {
      paramNames.push(name);
      return '([^/]+)';
    });

    this.routes.push({
      pattern: new RegExp(`^${regexPattern}$`),
      handler,
      paramNames,
    });

    return this;
  }

  /**
   * Register a handler for unmatched routes
   */
  notFound(handler: RouteHandler): this {
    this.notFoundHandler = handler;
    return this;
  }

  /**
   * Register a hook called before each navigation (after hash changes).
   * Receives (fromPath, toPath).
   */
  beforeNavigate(hook: (from: string, to: string) => void): this {
    this.beforeNavigateHooks.push(hook);
    return this;
  }

  /**
   * Register a hook called after each route handler completes.
   * Receives the resolved path.
   */
  afterNavigate(hook: (path: string) => void): this {
    this.afterNavigateHooks.push(hook);
    return this;
  }

  /**
   * Navigate to a path
   */
  navigate(path: string): void {
    window.location.hash = path;
  }

  /**
   * Get current path from hash
   */
  getCurrentPath(): string {
    const hash = window.location.hash.slice(1); // Remove #
    return hash || '/';
  }

  /**
   * Start listening for hash changes
   */
  start(): void {
    window.addEventListener('hashchange', () => this.handleRoute());
    this.handleRoute();
  }

  /**
   * Handle the current route
   */
  private async handleRoute(): Promise<void> {
    const path = this.getCurrentPath();
    const prevPath = (this as any)._lastPath as string | undefined;
    (this as any)._lastPath = path;

    for (const hook of this.beforeNavigateHooks) {
      hook(prevPath ?? '/', path);
    }

    for (const route of this.routes) {
      const match = path.match(route.pattern);
      if (match) {
        const params: Record<string, string> = {};
        route.paramNames.forEach((name, idx) => {
          params[name] = decodeURIComponent(match[idx + 1]);
        });
        await route.handler(params);

        for (const hook of this.afterNavigateHooks) {
          hook(path);
        }
        return;
      }
    }

    if (this.notFoundHandler) {
      await this.notFoundHandler({});
    }

    for (const hook of this.afterNavigateHooks) {
      hook(path);
    }
  }
}

export const router = new Router();
