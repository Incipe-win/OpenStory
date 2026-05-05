export const queryKeys = {
  auth: {
    me: ["auth", "me"] as const,
  },
  projects: {
    all: ["projects"] as const,
    list: (page: number, pageSize: number) =>
      ["projects", "list", { page, pageSize }] as const,
    detail: (id: string) => ["projects", id] as const,
    tasks: (id: string, page: number) =>
      ["projects", id, "tasks", page] as const,
    assets: (id: string, page: number) =>
      ["projects", id, "assets", page] as const,
  },
  workflows: {
    detail: (id: string) => ["workflows", id] as const,
  },
  tasks: {
    detail: (id: string) => ["tasks", id] as const,
    events: (id: string) => ["tasks", id, "events"] as const,
  },
  assets: {
    detail: (id: string) => ["assets", id] as const,
  },
  feed: {
    list: (page: number) => ["feed", page] as const,
  },
  credits: {
    balance: ["credits", "balance"] as const,
  },
  works: {
    detail: (id: string) => ["works", id] as const,
  },
  admin: {
    moderation: (status: string) => ["admin", "moderation", status] as const,
  },
} as const;
