<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import {
  createPermission,
  deletePermission,
  listPermissions,
  updatePermission,
  type RoutePermission
} from "@/api/users";
import { getApiErrorMessage } from "@/api/deploy";
import { message } from "@/utils/message";
import { getPermissionDisplayName } from "@/utils/permission-display-name";
import { ElMessageBox } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";

defineOptions({ name: "AccountsPermissions" });

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const editingId = ref<number | null>(null);
const permissions = ref<RoutePermission[]>([]);
const form = reactive({ code: "", name: "", path: "" });
const submitDisabled = computed(() => !form.code || !form.name || !form.path);

async function loadData() {
  loading.value = true;
  try {
    permissions.value = await listPermissions();
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载权限失败")), { type: "error" });
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = null;
  Object.assign(form, { code: "", name: "", path: "" });
  dialogVisible.value = true;
}

function openEdit(row: RoutePermission) {
  editingId.value = row.id;
  Object.assign(form, { code: row.code, name: row.name, path: row.path });
  dialogVisible.value = true;
}

async function submit() {
  saving.value = true;
  try {
    const payload = { ...form };
    if (editingId.value === null) {
      await createPermission(payload);
      message(tt("权限创建成功"), { type: "success" });
    } else {
      await updatePermission(editingId.value, payload);
      message(tt("权限修改成功"), { type: "success" });
    }
    dialogVisible.value = false;
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("保存权限失败")), { type: "error" });
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: RoutePermission) {
  try {
    await ElMessageBox.confirm(
      tt("确定删除权限“{name}”？", { name: row.name }),
      tt("删除权限"),
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
    await deletePermission(row.id);
    message(tt("删除成功"), { type: "success" });
    await loadData();
  } catch (error) {
    const response = (error as { response?: { data?: { roles?: string[] } } })
      ?.response;
    const roles = response?.data?.roles ?? [];
    const backendMessage = getApiErrorMessage(error, "");
    message(
      roles.length > 0
        ? tt("权限已分配给角色：{roles}，请先修改角色权限", {
            roles: roles.join("、")
          })
        : backendMessage ===
            "permission is assigned to roles; update role permissions first"
          ? tt("权限已分配给角色，请先修改角色权限")
          : backendMessage || tt("删除权限失败"),
      { type: "error" }
    );
  }
}

onMounted(loadData);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <p class="text-lg font-medium">{{ $tt("权限管理") }}</p>
          <p class="text-sm text-gray-500">
            {{ $tt("维护路由名称、权限标识和对应路由路径") }}
          </p>
        </div>
        <el-button type="primary" @click="openCreate">{{
          $tt("新建权限")
        }}</el-button>
      </div>
    </template>

    <el-table v-loading="loading" :data="permissions">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column :label="$tt('路由名称')" min-width="160">
        <template #default="{ row }">
          {{ getPermissionDisplayName(row.code, row.name) }}
        </template>
      </el-table-column>
      <el-table-column prop="code" :label="$tt('权限标识')" min-width="180" />
      <el-table-column prop="path" :label="$tt('路由路径')" min-width="220" />
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
    :title="editingId === null ? $tt('新建权限') : $tt('修改权限')"
    width="560px"
  >
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item :label="$tt('路由名称')" required>
        <el-input v-model="form.name" :placeholder="$tt('例如：项目管理')" />
      </el-form-item>
      <el-form-item :label="$tt('权限标识')" required>
        <el-input
          v-model="form.code"
          :placeholder="$tt('例如：project:view')"
        />
      </el-form-item>
      <el-form-item :label="$tt('路由路径')" required>
        <el-input
          v-model="form.path"
          :placeholder="$tt('例如：/deploy/projects')"
        />
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
