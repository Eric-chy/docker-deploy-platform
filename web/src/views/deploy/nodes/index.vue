<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import { message } from "@/utils/message";
import {
  createNode,
  deleteNode,
  getApiErrorMessage,
  listNodes,
  updateNode,
  type Node
} from "@/api/deploy";
import { ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

defineOptions({ name: "DeployNodes" });

const loading = ref(false);
const nodes = ref<Node[]>([]);
const dialogVisible = ref(false);
const form = reactive({
  name: "",
  address: "http://127.0.0.1:8120",
  apiKey: ""
});
const editingId = ref<number | null>(null);
const saving = ref(false);

async function loadData() {
  loading.value = true;
  try {
    nodes.value = await listNodes();
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载节点失败")), { type: "error" });
  } finally {
    loading.value = false;
  }
}

async function submit() {
  saving.value = true;
  try {
    if (editingId.value === null) {
      await createNode(form.name, form.address, form.apiKey);
      message(tt("创建成功"), { type: "success" });
    } else {
      await updateNode(editingId.value, {
        name: form.name,
        address: form.address,
        api_key: form.apiKey || undefined
      });
      message(tt("修改成功"), { type: "success" });
    }
    Object.assign(form, {
      name: "",
      address: "http://127.0.0.1:8120",
      apiKey: ""
    });
    editingId.value = null;
    dialogVisible.value = false;
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("保存节点失败")), { type: "error" });
  } finally {
    saving.value = false;
  }
}

function openCreate() {
  Object.assign(form, {
    name: "",
    address: "http://127.0.0.1:8120",
    apiKey: ""
  });
  editingId.value = null;
  dialogVisible.value = true;
}

function openEdit(row: Node) {
  Object.assign(form, {
    name: row.name,
    address: row.address,
    apiKey: ""
  });
  editingId.value = row.id;
  dialogVisible.value = true;
}

async function handleDelete(row: Node) {
  try {
    await ElMessageBox.confirm(
      tt("确定删除节点“{name}”？", { name: row.name }),
      tt("删除节点"),
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
    await deleteNode(row.id);
    message(tt("删除成功"), { type: "success" });
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("删除节点失败")), { type: "error" });
  }
}

onMounted(loadData);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <p class="text-lg font-medium">{{ $tt("节点管理") }}</p>
          <p class="text-sm text-gray-500">
            {{ $tt("每台目标机器运行一个节点服务") }}
          </p>
        </div>
        <el-button type="primary" @click="openCreate">{{
          $tt("新建节点")
        }}</el-button>
      </div>
    </template>
    <el-table v-loading="loading" :data="nodes">
      <el-table-column prop="id" label="ID" width="90" />
      <el-table-column prop="name" :label="$tt('节点名')" min-width="150" />
      <el-table-column
        prop="address"
        :label="$tt('节点地址')"
        min-width="220"
      />
      <el-table-column :label="$tt('状态')" width="110">
        <template #default="{ row }">
          <el-tag :type="row.status === 'online' ? 'success' : 'info'">
            {{ row.status }}
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
    :title="editingId === null ? $tt('新建节点') : $tt('修改节点')"
    width="520px"
  >
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item :label="$tt('节点名')" required>
        <el-input v-model="form.name" placeholder="local" />
      </el-form-item>
      <el-form-item :label="$tt('节点地址')" required>
        <el-input v-model="form.address" />
      </el-form-item>
      <el-form-item
        :label="
          editingId === null
            ? $tt('节点 API Key')
            : $tt('节点 API Key（留空则不修改）')
        "
      >
        <el-input v-model="form.apiKey" show-password type="password" />
      </el-form-item>
      <el-button
        native-type="submit"
        type="primary"
        :loading="saving"
        :disabled="
          !form.name ||
          !form.address ||
          (editingId === null && form.apiKey.length < 32)
        "
      >
        {{ editingId === null ? tt("创建") : tt("保存") }}
      </el-button>
    </el-form>
  </el-dialog>
</template>
