<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import {
  createUser,
  deleteUser,
  listRoles,
  listUsers,
  updateUser,
  type SystemUser
} from "@/api/users";
import { getApiErrorMessage, listProjects, type Project } from "@/api/deploy";
import { message } from "@/utils/message";
import { getPermissionDisplayName } from "@/utils/permission-display-name";
import { ElMessageBox } from "element-plus";
import type { FormInstance, FormItemRule } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";
import { validatePassword } from "@/utils/password";

defineOptions({ name: "AccountsUsers" });

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const editingUserId = ref<number | null>(null);
const users = ref<SystemUser[]>([]);
const projects = ref<Project[]>([]);
const roles = ref<Awaited<ReturnType<typeof listRoles>>>([]);
const selectedProjectIds = ref<number[]>([]);
const selectedRoleIds = ref<number[]>([]);
const form = reactive({
  username: "",
  password: "",
  is_admin: false,
  disabled: false
});
const formRef = ref<FormInstance>();

const passwordRules = computed<FormItemRule[]>(() => [
  {
    validator: (_rule, value: string, callback) => {
      if (!value && editingUserId.value === null) {
        callback(new Error(tt("请输入密码")));
        return;
      }
      if (value && !validatePassword(value)) {
        callback(
          new Error(tt("密码格式应为8-18位数字、字母、符号的任意两种组合"))
        );
        return;
      }
      callback();
    },
    trigger: ["blur", "change"]
  }
]);

const submitDisabled = computed(
  () =>
    !form.username ||
    (!form.is_admin && selectedProjectIds.value.length === 0) ||
    (!form.is_admin && selectedRoleIds.value.length === 0)
);

async function loadData() {
  loading.value = true;
  try {
    const [userData, projectData, roleData] = await Promise.all([
      listUsers(),
      listProjects(),
      listRoles()
    ]);
    users.value = userData;
    projects.value = projectData;
    roles.value = roleData;
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载账号失败")), { type: "error" });
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingUserId.value = null;
  Object.assign(form, {
    username: "",
    password: "",
    is_admin: false,
    disabled: false
  });
  selectedProjectIds.value = [];
  selectedRoleIds.value = [];
  dialogVisible.value = true;
}

function openEdit(row: SystemUser) {
  editingUserId.value = row.id;
  Object.assign(form, {
    username: row.username,
    password: "",
    is_admin: row.is_admin,
    disabled: row.disabled
  });
  selectedProjectIds.value = row.projects.map(project => project.project_id);
  selectedRoleIds.value = row.roles.map(role => role.id);
  dialogVisible.value = true;
}

async function submit() {
  await formRef.value?.validate();
  saving.value = true;
  try {
    const payload = {
      username: form.username,
      password: form.password || undefined,
      is_admin: form.is_admin,
      project_ids: [...selectedProjectIds.value],
      role_ids: [...selectedRoleIds.value]
    };
    if (editingUserId.value === null) {
      await createUser({ ...payload, password: form.password });
      message(tt("账号创建成功"), { type: "success" });
    } else {
      await updateUser(editingUserId.value, {
        ...payload,
        disabled: form.disabled
      });
      message(tt("账号修改成功"), { type: "success" });
    }
    dialogVisible.value = false;
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("保存账号失败")), { type: "error" });
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: SystemUser) {
  try {
    await ElMessageBox.confirm(
      tt("确定删除账号“{name}”？", { name: row.username }),
      tt("删除账号"),
      {
        type: "warning",
        confirmButtonText: tt("删除"),
        cancelButtonText: tt("取消")
      }
    );
  } catch {
    return;
  }
  try {
    await deleteUser(row.id);
    message(tt("删除成功"), { type: "success" });
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("删除账号失败")), { type: "error" });
  }
}

onMounted(loadData);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <p class="text-lg font-medium">{{ $tt("账号管理") }}</p>
          <p class="text-sm text-gray-500">
            {{ $tt("添加账号并绑定角色，保存时自动合并所选角色的路由权限") }}
          </p>
        </div>
        <el-button type="primary" @click="openCreate">{{
          $tt("新建账号")
        }}</el-button>
      </div>
    </template>

    <el-table v-loading="loading" :data="users">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" :label="$tt('用户名')" min-width="140" />
      <el-table-column :label="$tt('类型')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.is_admin ? 'danger' : 'info'">
            {{ row.is_admin ? "admin" : "member" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('状态')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.disabled ? 'danger' : 'success'">
            {{ row.disabled ? "disabled" : "active" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('角色')" min-width="180">
        <template #default="{ row }">
          <el-tag v-for="role in row.roles" :key="role.id" class="mr-2">
            {{ role.name }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('路由权限')" min-width="260">
        <template #default="{ row }">
          <el-tag
            v-for="permission in row.permissions"
            :key="permission.id"
            class="mr-2"
            type="info"
          >
            {{ getPermissionDisplayName(permission.code, permission.name) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('授权项目')" min-width="220">
        <template #default="{ row }">
          <el-tag
            v-for="project in row.projects"
            :key="project.project_id"
            class="mr-2"
            type="warning"
          >
            {{ project.name }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('操作')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">{{
            $tt("编辑")
          }}</el-button>
          <el-button size="small" type="danger" @click="handleDelete(row)">{{
            $tt("删除")
          }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog
    v-model="dialogVisible"
    :title="editingUserId === null ? $tt('新建账号') : $tt('修改账号')"
    width="720px"
  >
    <el-form
      ref="formRef"
      :model="form"
      label-position="top"
      @submit.prevent="submit"
    >
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item :label="$tt('用户名')" required>
            <el-input v-model="form.username" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item
            prop="password"
            :rules="passwordRules"
            :label="
              editingUserId === null ? $tt('密码') : $tt('密码（留空则不修改）')
            "
            :required="editingUserId === null"
          >
            <el-input v-model="form.password" show-password type="password" />
            <p class="mt-1 text-xs text-gray-500">
              {{ $tt("密码格式应为8-18位数字、字母、符号的任意两种组合") }}
            </p>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('全局管理员')">
            <el-switch v-model="form.is_admin" />
          </el-form-item>
        </el-col>
        <el-col v-if="editingUserId !== null" :span="12">
          <el-form-item :label="$tt('禁用')">
            <el-switch v-model="form.disabled" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item :label="$tt('角色')" required>
        <el-checkbox-group v-model="selectedRoleIds">
          <el-checkbox v-for="role in roles" :key="role.id" :value="role.id">
            {{ role.name }}（{{ role.code }}）
          </el-checkbox>
        </el-checkbox-group>
      </el-form-item>

      <el-form-item :label="$tt('项目')" :required="!form.is_admin">
        <el-checkbox-group v-model="selectedProjectIds">
          <el-checkbox
            v-for="project in projects"
            :key="project.id"
            :value="project.id"
          >
            {{ project.name }}
          </el-checkbox>
        </el-checkbox-group>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="dialogVisible = false">{{ $tt("取消") }}</el-button>
      <el-button
        type="primary"
        :loading="saving"
        :disabled="submitDisabled"
        @click="submit"
        >{{ $tt("保存") }}</el-button
      >
    </template>
  </el-dialog>
</template>
