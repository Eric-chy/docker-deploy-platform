<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import { message } from "@/utils/message";
import {
  createTeam,
  deleteTeam,
  getApiErrorMessage,
  listTeams,
  updateTeam,
  type Team
} from "@/api/deploy";
import { ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

defineOptions({ name: "DeployTeams" });

const loading = ref(false);
const teams = ref<Team[]>([]);
const dialogVisible = ref(false);
const form = reactive({ name: "" });
const editingId = ref<number | null>(null);
const saving = ref(false);

async function loadData() {
  loading.value = true;
  try {
    teams.value = await listTeams();
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载项目组失败")), { type: "error" });
  } finally {
    loading.value = false;
  }
}

async function submit() {
  saving.value = true;
  try {
    if (editingId.value === null) {
      await createTeam(form.name);
      message(tt("创建成功"), { type: "success" });
    } else {
      await updateTeam(editingId.value, form.name);
      message(tt("修改成功"), { type: "success" });
    }
    form.name = "";
    editingId.value = null;
    dialogVisible.value = false;
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("保存项目组失败")), { type: "error" });
  } finally {
    saving.value = false;
  }
}

function openCreate() {
  form.name = "";
  editingId.value = null;
  dialogVisible.value = true;
}

function openEdit(row: Team) {
  form.name = row.name;
  editingId.value = row.id;
  dialogVisible.value = true;
}

async function handleDelete(row: Team) {
  try {
    await ElMessageBox.confirm(
      tt("确定删除项目组“{name}”？", { name: row.name }),
      tt("删除项目组"),
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
    await deleteTeam(row.id);
    message(tt("删除成功"), { type: "success" });
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("删除项目组失败")), { type: "error" });
  }
}

onMounted(loadData);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <p class="text-lg font-medium">{{ $tt("项目组") }}</p>
          <p class="text-sm text-gray-500">
            {{ $tt("按团队隔离项目部署权限") }}
          </p>
        </div>
        <el-button type="primary" @click="openCreate">{{
          $tt("新建组")
        }}</el-button>
      </div>
    </template>
    <el-table v-loading="loading" :data="teams">
      <el-table-column prop="id" label="ID" width="100" />
      <el-table-column prop="name" :label="$tt('组名')" min-width="180" />
      <el-table-column
        prop="created_at"
        :label="$tt('创建时间')"
        min-width="180"
      />
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
    :title="editingId === null ? $tt('新建项目组') : $tt('修改项目组')"
    width="420px"
  >
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item :label="$tt('组名')" required>
        <el-input v-model="form.name" placeholder="dev" />
      </el-form-item>
      <el-button
        native-type="submit"
        type="primary"
        :loading="saving"
        :disabled="form.name.trim() === ''"
      >
        {{ editingId === null ? tt("创建") : tt("保存") }}
      </el-button>
    </el-form>
  </el-dialog>
</template>
