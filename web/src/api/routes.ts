export type RouteResult = {
  success: boolean;
  data: Array<any>;
};

export const getAsyncRoutes = (): Promise<RouteResult> =>
  Promise.resolve({ success: true, data: [] });
