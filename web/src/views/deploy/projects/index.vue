<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import { message } from "@/utils/message";
import {
  createProject,
  deleteProject,
  deploy,
  getProject,
  listDeployments,
  listNodes,
  listProjects,
  listTeams,
  rollbackDeployment,
  retryDeployment,
  updateProject,
  getApiErrorMessage,
  type Deployment,
  type Node,
  type Project,
  type Team
} from "@/api/deploy";
import { ElMessageBox } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";
import { hasPerms } from "@/utils/auth";
import { validateComposeYaml } from "@/utils/compose-yaml";
import YamlEditor from "@/components/YamlEditor/index.vue";

defineOptions({ name: "DeployProjects" });

const loading = ref(false);
const projects = ref<Project[]>([]);
const teams = ref<Team[]>([]);
const nodes = ref<Node[]>([]);
const createVisible = ref(false);
const editVisible = ref(false);
const deployVisible = ref(false);
const historyVisible = ref(false);
const historyLoading = ref(false);
const deployLoading = ref(false);
const deployments = ref<Deployment[]>([]);
const historyPage = ref(1);
const historyTotal = ref(0);
const historyActionLoadingId = ref<number | null>(null);
const editLoading = ref(false);

const createForm = reactive({
  name: "",
  team_id: undefined as number | undefined,
  node_id: undefined as number | undefined,
  current_yaml:
    'services:\n  app:\n    image: nginx:alpine\n    ports:\n      - "8088:80"\n',
  current_version: "v0"
});

const deployForm = reactive({
  projectId: 0,
  name: "",
  version: "",
  yaml: ""
});

const editForm = reactive({
  id: 0,
  name: "",
  team_id: undefined as number | undefined,
  node_id: undefined as number | undefined,
  current_yaml: ""
});

const historyProject = ref<Project | null>(null);
const createDisabled = computed(
  () =>
    !createForm.name ||
    createForm.team_id === undefined ||
    createForm.node_id === undefined ||
    !createForm.current_yaml.trim() ||
    validateComposeYaml(createForm.current_yaml).some(
      diagnostic => diagnostic.severity === "error"
    )
);
const deployDisabled = computed(
  () =>
    !deployForm.yaml.trim() ||
    validateComposeYaml(deployForm.yaml).some(
      diagnostic => diagnostic.severity === "error"
    )
);
const editDisabled = computed(
  () =>
    !editForm.name ||
    editForm.team_id === undefined ||
    editForm.node_id === undefined ||
    !editForm.current_yaml.trim() ||
    validateComposeYaml(editForm.current_yaml).some(
      diagnostic => diagnostic.severity === "error"
    )
);
const canManageProject = computed(() => hasPerms("project:create"));
const canDeployProject = computed(() => hasPerms("deploy:publish"));

async function loadData() {
  loading.value = true;
  try {
    const [projectData, teamData, nodeData] = await Promise.all([
      listProjects(),
      listTeams(),
      listNodes()
    ]);
    projects.value = projectData;
    teams.value = teamData;
    nodes.value = nodeData;
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载项目数据失败")), {
      type: "error"
    });
  } finally {
    loading.value = false;
  }
}

async function submitCreate() {
  if (createDisabled.value) {
    message(tt("请先完善项目信息并修复 YAML 配置错误"), { type: "error" });
    return;
  }
  try {
    await createProject({
      name: createForm.name,
      team_id: createForm.team_id!,
      node_id: createForm.node_id!,
      current_yaml: createForm.current_yaml,
      current_version: createForm.current_version
    });
    message(tt("项目创建成功"), { type: "success" });
    Object.assign(createForm, {
      name: "",
      team_id: undefined,
      node_id: undefined,
      current_yaml:
        'services:\n  app:\n    image: nginx:alpine\n    ports:\n      - "8088:80"\n',
      current_version: "v0"
    });
    createVisible.value = false;
    await loadData();
  } catch (error) {
    showApiError(error);
  }
}

async function openEdit(row: Project) {
  try {
    const detail = await getProject(row.id);
    Object.assign(editForm, {
      id: row.id,
      name: detail.name,
      team_id: detail.team_id,
      node_id: detail.node_id,
      current_yaml: detail.current_yaml
    });
    editVisible.value = true;
  } catch (error) {
    showApiError(error);
  }
}

async function submitEdit() {
  if (editDisabled.value) {
    message(tt("请先完善项目信息并修复 YAML 配置错误"), { type: "error" });
    return;
  }
  editLoading.value = true;
  try {
    await updateProject(editForm.id, {
      name: editForm.name,
      team_id: editForm.team_id!,
      node_id: editForm.node_id!,
      current_yaml: editForm.current_yaml
    });
    message(tt("项目修改成功"), { type: "success" });
    editVisible.value = false;
    await loadData();
  } catch (error) {
    showApiError(error);
  } finally {
    editLoading.value = false;
  }
}

async function handleDelete(row: Project) {
  try {
    await ElMessageBox.confirm(
      tt(
        "确定删除项目“{name}”？发布历史会一并删除，节点上的服务不会被自动移除。",
        {
          name: row.name
        }
      ),
      tt("删除项目"),
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
    await deleteProject(row.id);
    message(tt("项目删除成功"), { type: "success" });
    await loadData();
  } catch (error) {
    showApiError(error);
  }
}

async function openDeploy(row: Project) {
  try {
    const detail = await getProject(row.id);
    Object.assign(deployForm, {
      projectId: row.id,
      name: row.name,
      version: row.current_version,
      yaml: detail.current_yaml
    });
    deployVisible.value = true;
  } catch (error) {
    showApiError(error);
  }
}

async function submitDeploy() {
  if (deployDisabled.value) {
    message(tt("请先修复 YAML 配置错误"), { type: "error" });
    return;
  }
  deployLoading.value = true;
  try {
    const result = await deploy(deployForm.projectId, deployForm.yaml);
    message(tt("发布已提交审批：{taskId}", { taskId: result.task_id }), {
      type: "success"
    });
    deployVisible.value = false;
    await loadData();
  } catch (error) {
    showApiError(error);
  } finally {
    deployLoading.value = false;
  }
}

async function openHistory(row: Project) {
  historyProject.value = row;
  historyVisible.value = true;
  historyPage.value = 1;
  await loadHistory();
}

async function loadHistory() {
  historyLoading.value = true;
  try {
    const result = await listDeployments(
      historyProject.value!.id,
      historyPage.value
    );
    deployments.value = result.items;
    historyTotal.value = result.total;
  } catch (error) {
    historyVisible.value = false;
    showApiError(error);
  } finally {
    historyLoading.value = false;
  }
}

async function handleHistoryPageChange(nextPage: number) {
  historyPage.value = nextPage;
  await loadHistory();
}

async function handleHistoryRollback(row: Deployment) {
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
  historyActionLoadingId.value = row.id;
  try {
    const result = await rollbackDeployment(row.id);
    message(tt("回滚已受理：{taskId}", { taskId: result.task_id }), {
      type: "success"
    });
    await loadHistory();
  } catch (error) {
    showApiError(error);
  } finally {
    historyActionLoadingId.value = null;
  }
}

async function handleHistoryRetry(row: Deployment) {
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
  historyActionLoadingId.value = row.id;
  try {
    const result = await retryDeployment(row.id);
    message(tt("重新发布已受理：{taskId}", { taskId: result.task_id }), {
      type: "success"
    });
    await loadHistory();
  } catch (error) {
    showApiError(error);
  } finally {
    historyActionLoadingId.value = null;
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

function statusType(status: string) {
  if (status === "success") return "success";
  if (status === "failed") return "danger";
  if (status === "partial" || status === "failed_recovered") return "warning";
  return "info";
}

function showApiError(error: unknown) {
  message(getApiErrorMessage(error, tt("操作失败")), { type: "error" });
}

onMounted(loadData);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <p class="text-lg font-medium">{{ $tt("项目管理") }}</p>
          <p class="text-sm text-gray-500">
            {{ $tt("维护 Compose 配置并发起发布") }}
          </p>
        </div>
        <el-button
          v-if="canManageProject"
          type="primary"
          @click="createVisible = true"
          >{{ $tt("新建项目") }}</el-button
        >
      </div>
    </template>

    <el-table v-loading="loading" :data="projects">
      <el-table-column prop="id" label="ID" width="90" />
      <el-table-column prop="name" :label="$tt('项目')" min-width="180" />
      <el-table-column
        prop="created_at"
        :label="$tt('创建时间')"
        min-width="180"
      />
      <el-table-column
        prop="current_version"
        :label="$tt('当前版本')"
        min-width="170"
      />
      <el-table-column
        prop="latest_deployed_at"
        :label="$tt('最新发布时间')"
        min-width="180"
      />
      <el-table-column :label="$tt('操作')" width="290" fixed="right">
        <template #default="{ row }">
          <div class="table-actions">
            <el-button size="small" @click="openHistory(row)">{{
              $tt("历史")
            }}</el-button>
            <el-button
              v-if="canManageProject"
              size="small"
              @click="openEdit(row)"
              >{{ $tt("编辑") }}</el-button
            >
            <el-button
              v-if="canDeployProject"
              size="small"
              type="primary"
              @click="openDeploy(row)"
              >{{ $tt("发布") }}</el-button
            >
            <el-button
              v-if="canManageProject"
              size="small"
              type="danger"
              @click="handleDelete(row)"
              >{{ $tt("删除") }}</el-button
            >
          </div>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="createVisible" :title="$tt('新建项目')" width="760px">
    <el-form label-position="top" @submit.prevent="submitCreate">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item :label="$tt('项目名')" required>
            <el-input v-model="createForm.name" placeholder="my-app" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('当前版本')">
            <el-input v-model="createForm.current_version" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('所属组')" required>
            <el-select v-model="createForm.team_id" class="w-full">
              <el-option
                v-for="team in teams"
                :key="team.id"
                :label="team.name"
                :value="team.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('目标节点')" required>
            <el-select v-model="createForm.node_id" class="w-full">
              <el-option
                v-for="node in nodes"
                :key="node.id"
                :label="`${node.name} (${node.address})`"
                :value="node.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item :label="$tt('初始 Compose YAML')" required>
        <YamlEditor v-model="createForm.current_yaml" height="360px" />
      </el-form-item>
      <el-button
        native-type="submit"
        type="primary"
        :disabled="createDisabled"
        >{{ $tt("创建") }}</el-button
      >
    </el-form>
  </el-dialog>

  <el-dialog v-model="editVisible" :title="$tt('修改项目')" width="760px">
    <el-form label-position="top" @submit.prevent="submitEdit">
      <el-alert
        :title="
          $tt('修改 YAML 只更新配置基线，不会部署节点；发布会生成发布记录。')
        "
        type="info"
        :closable="false"
        class="mb-4"
      />
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item :label="$tt('项目名')" required>
            <el-input v-model="editForm.name" placeholder="my-app" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('所属组')" required>
            <el-select v-model="editForm.team_id" class="w-full">
              <el-option
                v-for="team in teams"
                :key="team.id"
                :label="team.name"
                :value="team.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('目标节点')" required>
            <el-select v-model="editForm.node_id" class="w-full">
              <el-option
                v-for="node in nodes"
                :key="node.id"
                :label="`${node.name} (${node.address})`"
                :value="node.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item :label="$tt('当前 Compose YAML')" required>
        <YamlEditor v-model="editForm.current_yaml" height="360px" />
      </el-form-item>
      <el-button
        native-type="submit"
        type="primary"
        :loading="editLoading"
        :disabled="editDisabled"
        >{{ $tt("保存") }}</el-button
      >
    </el-form>
  </el-dialog>

  <el-dialog
    v-model="deployVisible"
    :title="$tt('发布 {name}', { name: deployForm.name })"
    width="820px"
  >
    <el-alert
      :title="$tt('当前版本：{version}', { version: deployForm.version })"
      type="info"
      :closable="false"
      class="mb-4"
    />
    <YamlEditor v-model="deployForm.yaml" height="420px" />
    <template #footer>
      <el-button @click="deployVisible = false">{{ $tt("取消") }}</el-button>
      <el-button
        type="primary"
        :loading="deployLoading"
        :disabled="deployDisabled"
        @click="submitDeploy"
        >{{ $tt("提交发布") }}</el-button
      >
    </template>
  </el-dialog>

  <el-dialog
    v-model="historyVisible"
    :title="$tt('发布历史：{name}', { name: historyProject?.name ?? '' })"
    width="1080px"
  >
    <el-table v-loading="historyLoading" :data="deployments">
      <el-table-column prop="id" label="ID" width="90" />
      <el-table-column prop="version" :label="$tt('版本')" min-width="170" />
      <el-table-column
        prop="created_at"
        :label="$tt('发布时间')"
        min-width="180"
      />
      <el-table-column prop="action" :label="$tt('类型')" width="100" />
      <el-table-column :label="$tt('状态')" width="150">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('耗时')" width="120">
        <template #default="{ row }">{{ row.duration_ms }} ms</template>
      </el-table-column>
      <el-table-column :label="$tt('操作')" width="190" fixed="right">
        <template #default="{ row }">
          <div class="table-actions">
            <el-button
              v-if="canDeployProject"
              size="small"
              type="warning"
              :loading="historyActionLoadingId === row.id"
              :disabled="!canRollback(row)"
              @click="handleHistoryRollback(row)"
              >{{ $tt("回滚") }}</el-button
            >
            <el-button
              v-if="canDeployProject"
              size="small"
              type="primary"
              :loading="historyActionLoadingId === row.id"
              :disabled="!canRetry(row)"
              @click="handleHistoryRetry(row)"
              >{{ $tt("重发") }}</el-button
            >
          </div>
        </template>
      </el-table-column>
    </el-table>
    <div class="mt-4 flex justify-end">
      <el-pagination
        background
        layout="total, prev, pager, next"
        :total="historyTotal"
        :page-size="50"
        :current-page="historyPage"
        @current-change="handleHistoryPageChange"
      />
    </div>
  </el-dialog>
</template>
<style scoped>
.table-actions {
  display: flex;
  max-width: 100%;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
}

.table-actions :deep(.el-button) {
  flex-shrink: 0;
  margin-left: 0;
}
</style>
