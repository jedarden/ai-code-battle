// Simple hash-based router for single-page navigation

export type RouteHandler = (params: Record<string, string>) => void | Promise<void>;

interface Route {
  pattern: RegExp;
  handler: RouteHandler;
  paramNames: string[];
}

class Router {
  private routes: Route[] = [];
  private notFoundHandler: RouteHandler | null = null;

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
  private handleRoute(): void {
    const path = this.getCurrentPath();

    for (const route of this.routes) {
      const match = path.match(route.pattern);
      if (match) {
        const params: Record<string, string> = {};
        route.paramNames.forEach((name, idx) => {
          params[name] = decodeURIComponent(match[idx + 1]);
        });
        route.handler(params);
        return;
      }
    }

    if (this.notFoundHandler) {
      this.notFoundHandler({});
    }
  }
}

export const router = new Router();
