import { http } from "@/utils/http";
import type { DataInfo } from "@/utils/auth";

export type UserResult = {
  success: boolean;
  data: DataInfo<Date>;
};

export type RefreshTokenResult = {
  success: boolean;
  data: DataInfo<Date>;
};

export type LoginPayload = {
  username: string;
  password: string;
};

type LoginResponse = {
  access_token: string;
  roles: string[];
  permissions: string[];
};

export async function getLogin(data: LoginPayload): Promise<UserResult> {
  const result = await http.request<LoginResponse>("post", "/api/auth/login", {
    data
  });

  return {
    success: true,
    data: {
      accessToken: result.access_token,
      refreshToken: "",
      expires: new Date(Date.now() + 29 * 60 * 1000),
      username: data.username,
      nickname: data.username,
      roles: result.roles,
      permissions: result.permissions
    }
  };
}

export const refreshTokenApi = async (_data?: object): Promise<never> => {
  throw new Error("登录已过期，请重新登录");
};
