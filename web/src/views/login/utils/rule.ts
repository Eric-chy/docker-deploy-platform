import { reactive } from "vue";
import type { FormRules } from "element-plus";
import { translate as tt } from "@/plugins/i18n";

import { validatePassword } from "@/utils/password";

/** 登录校验 */
const loginRules = reactive<FormRules>({
  password: [
    {
      validator: (rule, value, callback) => {
        if (value === "") {
          callback(new Error(tt("请输入密码")));
        } else if (!validatePassword(value)) {
          callback(
            new Error(tt("密码格式应为8-18位数字、字母、符号的任意两种组合"))
          );
        } else {
          callback();
        }
      },
      trigger: "blur"
    }
  ]
});

export { loginRules };
