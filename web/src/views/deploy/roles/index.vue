<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import {
  createRole,
  deleteRole,
  listPermissions,
  listRoles,
  updateRole,
  type Role,
  type RoutePermission
} from "@/api/users";
import { getApiErrorMessage } from "@/api/deploy";
import { message } from "@/utils/message";
import { getPermissionDisplayName } from "@/utils/permission-display-name";
import { ElMessageBox } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";

defineOptions({ name: "AccountsRoles" });

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const editingId = ref<number | null>(null);
const roles = ref<Role[]>([]);
const permissions = ref<RoutePermission[]>([]);
const form = reactive({
  code: "",
  name: "",
  permission_ids: [] as number[]
});
const submitDisabled = computed(
  () => !form.code || !form.name || form.permission_ids.length === 0
);

async function loadData() {
  loading.value = true;
  try {
    const [roleData, permissionData] = await Promise.all([
      listRoles(),
      listPermissions()
    ]);
    roles.value = roleData;
    permissions.value = permissionData;
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载角色失败")), { type: "error" });
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = null;
  Object.assign(form, {
    code: "",
    name: "",
    permission_ids: []
  });
  dialogVisible.value = true;
}

function openEdit(row: Role) {
  editingId.value = row.id;
  Object.assign(form, {
    code: row.code,
    name: row.name,
    permission_ids: [...row.permission_ids]
  });
  dialogVisible.value = true;
}

async function submit() {
  saving.value = true;
  try {
    const payload = { ...form };
    if (editingId.value === null) {
      await createRole(payload);
      message(tt("角色创建成功"), { type: "success" });
    } else {
      await updateRole(editingId.value, payload);
      message(tt("角色修改成功"), { type: "success" });
    }
    dialogVisible.value = false;
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("保存角色失败")), { type: "error" });
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: Role) {
  try {
    await ElMessageBox.confirm(
      tt("确定删除角色“{name}”？", { name: row.name }),
      tt("删除角色"),
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
    await deleteRole(row.id);
    message(tt("删除成功"), { type: "success" });
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("删除角色失败")), { type: "error" });
  }
}

onMounted(loadData);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <p class="text-lg font-medium">{{ $tt("角色管理") }}</p>
          <p class="text-sm text-gray-500">
            {{ $tt("创建角色并勾选可访问的路由权限") }}
          </p>
        </div>
        <el-button type="primary" @click="openCreate">{{
          $tt("新建角色")
        }}</el-button>
      </div>
    </template>

    <el-table v-loading="loading" :data="roles">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="code" :label="$tt('角色标识')" min-width="140" />
      <el-table-column prop="name" :label="$tt('角色名称')" min-width="140" />
      <el-table-column :label="$tt('路由权限')" min-width="300">
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
    :title="editingId === null ? $tt('新建角色') : $tt('修改角色')"
    width="620px"
  >
    <el-form label-position="top" @submit.prevent="submit">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item :label="$tt('角色标识')" required>
            <el-input
              v-model="form.code"
              :placeholder="$tt('例如：developer')"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('角色名称')" required>
            <el-input
              v-model="form.name"
              :placeholder="$tt('例如：开发人员')"
            />
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item :label="$tt('路由权限')" required>
        <el-checkbox-group v-model="form.permission_ids">
          <el-checkbox
            v-for="permission in permissions"
            :key="permission.id"
            :value="permission.id"
          >
            {{ getPermissionDisplayName(permission.code, permission.name) }}（{{
              permission.path
            }}）
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
