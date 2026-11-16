<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import {
  approveDeployment,
  getApiErrorMessage,
  getDeploymentDetail,
  rejectDeployment,
  rollbackDeployment,
  retryDeployment,
  listAllDeployments,
  type Deployment,
  type DeploymentDetail
} from "@/api/deploy";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { hasPerms } from "@/utils/auth";

defineOptions({ name: "DeployHistory" });

const loading = ref(false);
const deployments = ref<Deployment[]>([]);
const total = ref(0);
const page = ref(1);
const actionLoadingId = ref<number | null>(null);
const detailVisible = ref(false);
const detailLoading = ref(false);
const selectedDeployment = ref<DeploymentDetail | null>(null);
const canDeployProject = computed(() => hasPerms("deploy:publish"));
const canApproveDeployment = computed(() => hasPerms("deploy:approve"));
const pollTimer = window.setInterval(() => {
  if (
    !loading.value &&
    actionLoadingId.value === null &&
    deployments.value.some(
      deployment => deployment.status === "pending_approval"
    )
  ) {
    void loadHistory(false);
  }
}, 5000);
const route = useRoute();
const router = useRouter();

function statusType(status: string) {
  if (status === "success") return "success";
  if (status === "failed") return "danger";
  if (
    status === "partial" ||
    status === "failed_recovered" ||
    status === "pending_approval"
  ) {
    return "warning";
  }
  return "info";
}

async function loadHistory(showLoading = true) {
  if (showLoading) loading.value = true;
  try {
    const result = await listAllDeployments(page.value);
    deployments.value = result.items;
    total.value = result.total;
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载发布历史失败")), {
      type: "error"
    });
  } finally {
    if (showLoading) loading.value = false;
  }
}

onUnmounted(() => window.clearInterval(pollTimer));

async function handlePageChange(nextPage: number) {
  page.value = nextPage;
  await loadHistory();
}

async function openDetail(row: Deployment) {
  detailVisible.value = true;
  detailLoading.value = true;
  selectedDeployment.value = null;
  try {
    selectedDeployment.value = await getDeploymentDetail(row.id);
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载发布详情失败")), {
      type: "error"
    });
    detailVisible.value = false;
  } finally {
    detailLoading.value = false;
  }
}

async function handleRollback(row: Deployment) {
  try {
    await ElMessageBox.confirm(
      tt(
        "将回滚到记录 #{id} 发布前的配置，并生成一条新的回滚记录。是否继续？",
        {
          id: row.id
        }
      ),
      tt("确认回滚"),
      {
        type: "warning",
        confirmButtonText: tt("回滚"),
        cancelButtonText: tt("取消")
      }
    );
  } catch {
    return;
  }
  actionLoadingId.value = row.id;
  try {
    const result = await rollbackDeployment(row.id);
    message(tt("回滚已受理：{taskId}", { taskId: result.task_id }), {
      type: "success"
    });
    await loadHistory();
  } catch (error) {
    message(getApiErrorMessage(error, tt("回滚失败")), { type: "error" });
  } finally {
    actionLoadingId.value = null;
  }
}

async function handleRetry(row: Deployment) {
  try {
    await ElMessageBox.confirm(
      tt(
        "将重新发布记录 #{id} 的目标配置，并生成一条新的发布记录。是否继续？",
        {
          id: row.id
        }
      ),
      tt("确认重新发布"),
      {
        type: "warning",
        confirmButtonText: tt("重新发布"),
        cancelButtonText: tt("取消")
      }
    );
  } catch {
    return;
  }
  actionLoadingId.value = row.id;
  try {
    const result = await retryDeployment(row.id);
    message(tt("重新发布已受理：{taskId}", { taskId: result.task_id }), {
      type: "success"
    });
    await loadHistory();
  } catch (error) {
    message(getApiErrorMessage(error, tt("重新发布失败")), { type: "error" });
  } finally {
    actionLoadingId.value = null;
  }
}

async function promptApproval(action: "approve" | "reject") {
  try {
    const { value } = await ElMessageBox.prompt(
      action === "approve"
        ? tt("请输入审批备注（可选）")
        : tt("请输入拒绝原因（可选）"),
      action === "approve" ? tt("同意发布") : tt("拒绝发布"),
      {
        confirmButtonText: action === "approve" ? tt("同意") : tt("拒绝"),
        cancelButtonText: tt("取消"),
        inputType: "textarea"
      }
    );
    return value || "";
  } catch {
    return null;
  }
}

async function handleApproval(row: Deployment, action: "approve" | "reject") {
  const comment = await promptApproval(action);
  if (comment === null) return;
  actionLoadingId.value = row.id;
  try {
    if (action === "approve") {
      const result = await approveDeployment(row.id, comment);
      message(tt("发布已开始：{taskId}", { taskId: result.task_id }), {
        type: "success"
      });
    } else {
      await rejectDeployment(row.id, comment);
      message(tt("发布已拒绝"), { type: "warning" });
    }
    await loadHistory();
  } catch (error) {
    message(
      getApiErrorMessage(
        error,
        action === "approve" ? tt("审批失败") : tt("拒绝失败")
      ),
      { type: "error" }
    );
  } finally {
    actionLoadingId.value = null;
  }
}

function canRollback(row: Deployment) {
  return (
    row.status !== "running" &&
    row.status !== "pending" &&
    row.status !== "pending_approval"
  );
}

function canRetry(row: Deployment) {
  return (
    row.status !== "running" &&
    row.status !== "pending" &&
    row.status !== "pending_approval" &&
    row.status !== "success"
  );
}

function canApprove(row: Deployment) {
  return row.status === "pending_approval";
}

async function handleNotificationAction() {
  const deploymentID = Number(route.query.deployment_id);
  const action = route.query.action;
  if (
    !Number.isInteger(deploymentID) ||
    (action !== "approve" && action !== "reject")
  ) {
    return;
  }
  const row = deployments.value.find(item => item.id === deploymentID);
  await router.replace({ path: route.path });
  if (row && canApprove(row)) {
    await handleApproval(row, action);
  }
}

onMounted(async () => {
  await loadHistory();
  await handleNotificationAction();
});
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div>
        <p class="text-lg font-medium">{{ $tt("发布历史") }}</p>
        <p class="text-sm text-gray-500">{{ $tt("部署与回滚记录") }}</p>
      </div>
    </template>
    <el-table v-loading="loading" :data="deployments">
      <el-table-column prop="id" label="ID" width="90" />
      <el-table-column prop="project" :label="$tt('项目')" min-width="140" />
      <el-table-column prop="version" :label="$tt('版本')" min-width="170" />
      <el-table-column
        prop="created_at"
        :label="$tt('发布时间')"
        min-width="180"
      />
      <el-table-column prop="action" :label="$tt('类型')" width="100" />
      <el-table-column prop="operator" :label="$tt('申请人')" min-width="120" />
      <el-table-column prop="approver" :label="$tt('审批人')" min-width="120" />
      <el-table-column :label="$tt('状态')" width="150">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('耗时')" width="120">
        <template #default="{ row }">{{ row.duration_ms }} ms</template>
      </el-table-column>
      <el-table-column :label="$tt('操作')" width="300" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="canApproveDeployment && canApprove(row)"
            size="small"
            type="success"
            :loading="actionLoadingId === row.id"
            @click="handleApproval(row, 'approve')"
            >{{ $tt("同意") }}</el-button
          >
          <el-button
            v-if="canApproveDeployment && canApprove(row)"
            size="small"
            type="danger"
            :loading="actionLoadingId === row.id"
            @click="handleApproval(row, 'reject')"
            >{{ $tt("拒绝") }}</el-button
          >
          <el-button
            v-if="canDeployProject"
            size="small"
            type="warning"
            :loading="actionLoadingId === row.id"
            :disabled="!canRollback(row)"
            @click="handleRollback(row)"
            >{{ $tt("回滚") }}</el-button
          >
          <el-button
            v-if="canDeployProject"
            size="small"
            type="primary"
            :loading="actionLoadingId === row.id"
            :disabled="!canRetry(row)"
            @click="handleRetry(row)"
            >{{ $tt("重发") }}</el-button
          >
          <el-button size="small" @click="openDetail(row)">{{
            $tt("详情")
          }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <div class="mt-4 flex justify-end">
      <el-pagination
        background
        layout="total, prev, pager, next"
        :total="total"
        :page-size="50"
        :current-page="page"
        @current-change="handlePageChange"
      />
    </div>
  </el-card>

  <el-dialog v-model="detailVisible" :title="$tt('发布详情')" width="860px">
    <div v-loading="detailLoading">
      <el-descriptions v-if="selectedDeployment" :column="2" border>
        <el-descriptions-item label="ID">
          {{ selectedDeployment.id }}
        </el-descriptions-item>
        <el-descriptions-item :label="$tt('任务号')">
          {{ selectedDeployment.task_id }}
        </el-descriptions-item>
        <el-descriptions-item :label="$tt('项目')">
          {{ selectedDeployment.project }}
        </el-descriptions-item>
        <el-descriptions-item :label="$tt('版本')">
          {{ selectedDeployment.version }}
        </el-descriptions-item>
        <el-descriptions-item :label="$tt('状态')">
          {{ selectedDeployment.status }}
        </el-descriptions-item>
        <el-descriptions-item :label="$tt('耗时')">
          {{ selectedDeployment.duration_ms }} ms
        </el-descriptions-item>
        <el-descriptions-item :label="$tt('申请人')">
          {{ selectedDeployment.operator ?? "-" }}
        </el-descriptions-item>
        <el-descriptions-item :label="$tt('审批人')">
          {{ selectedDeployment.approver ?? "-" }}
        </el-descriptions-item>
      </el-descriptions>

      <el-alert
        v-if="selectedDeployment?.approval_comment"
        class="mb-3"
        :title="
          $tt('审批备注：{comment}', {
            comment: selectedDeployment.approval_comment
          })
        "
        type="info"
        :closable="false"
      />

      <div v-if="selectedDeployment">
        <p class="mb-2 font-medium">{{ $tt("执行输出") }}</p>
        <el-scrollbar max-height="420px">
          <pre
            class="whitespace-pre-wrap break-all rounded bg-gray-100 p-3 text-xs dark:bg-gray-900"
            >{{ selectedDeployment.output || tt("暂无输出") }}</pre
          >
        </el-scrollbar>
      </div>
    </div>
  </el-dialog>
</template>
