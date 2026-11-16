<script setup lang="ts">
import { translate as tt } from "@/plugins/i18n";
import {
  createNotificationChannel,
  deleteNotificationChannel,
  listNotificationChannels,
  updateNotificationChannel,
  type NotificationChannel
} from "@/api/notifications";
import { getApiErrorMessage, listTeams, type Team } from "@/api/deploy";
import { listRoles, type Role } from "@/api/users";
import { message } from "@/utils/message";
import { ElMessageBox } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";

defineOptions({ name: "NotificationChannels" });

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const editingId = ref<number | null>(null);
const channels = ref<NotificationChannel[]>([]);
const roles = ref<Role[]>([]);
const teams = ref<Team[]>([]);
const selectedTeamIds = ref(new Set<number>());
const form = reactive({
  role_ids: [] as number[],
  type: "dingtalk",
  name: "",
  webhook_url: "",
  secret: "",
  enabled: true
});

const typeOptions = [
  { label: tt("钉钉"), value: "dingtalk" },
  { label: tt("飞书"), value: "feishu" },
  { label: tt("企业微信"), value: "wecom" }
];

const submitDisabled = computed(
  () =>
    !form.type ||
    !form.name ||
    !form.webhook_url ||
    selectedTeamIds.value.size === 0
);

async function loadData() {
  loading.value = true;
  try {
    const [channelData, roleData, teamData] = await Promise.all([
      listNotificationChannels(),
      listRoles(),
      listTeams()
    ]);
    channels.value = channelData;
    roles.value = roleData;
    teams.value = teamData;
  } catch (error) {
    message(getApiErrorMessage(error, tt("加载通知渠道失败")), {
      type: "error"
    });
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = null;
  Object.assign(form, {
    role_ids: [],
    type: "dingtalk",
    name: "",
    webhook_url: "",
    secret: "",
    enabled: true
  });
  selectedTeamIds.value = new Set();
  dialogVisible.value = true;
}

function openEdit(row: NotificationChannel) {
  editingId.value = row.id;
  Object.assign(form, {
    role_ids: row.role_ids,
    type: row.type,
    name: row.name,
    webhook_url: row.webhook_url,
    secret: "",
    enabled: row.enabled
  });
  selectedTeamIds.value = new Set(row.team_ids);
  dialogVisible.value = true;
}

async function submit() {
  saving.value = true;
  try {
    const payload = {
      ...form,
      role_ids: form.role_ids,
      team_ids: Array.from(selectedTeamIds.value),
      secret: form.secret || undefined
    };
    if (editingId.value === null) {
      await createNotificationChannel(payload);
      message(tt("通知渠道创建成功"), { type: "success" });
    } else {
      await updateNotificationChannel(editingId.value, payload);
      message(tt("通知渠道修改成功"), { type: "success" });
    }
    dialogVisible.value = false;
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("保存通知渠道失败")), {
      type: "error"
    });
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: NotificationChannel) {
  try {
    await ElMessageBox.confirm(
      tt("确定删除通知渠道“{name}”？", { name: row.name }),
      tt("删除通知渠道"),
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
    await deleteNotificationChannel(row.id);
    message(tt("删除成功"), { type: "success" });
    await loadData();
  } catch (error) {
    message(getApiErrorMessage(error, tt("删除通知渠道失败")), {
      type: "error"
    });
  }
}

onMounted(loadData);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <p class="text-lg font-medium">{{ $tt("通知渠道") }}</p>
          <p class="text-sm text-gray-500">
            {{
              $tt(
                "配置钉钉、飞书、企业微信 Webhook；签名密钥仅用于服务端发送通知"
              )
            }}
          </p>
        </div>
        <el-button type="primary" @click="openCreate">{{
          $tt("新建渠道")
        }}</el-button>
      </div>
    </template>

    <el-table v-loading="loading" :data="channels">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" :label="$tt('名称')" min-width="140" />
      <el-table-column :label="$tt('接收角色')" min-width="160">
        <template #default="{ row }">
          <el-tag
            v-for="roleName in row.role_names"
            :key="roleName"
            class="mr-2"
            type="info"
          >
            {{ roleName }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('类型')" width="120">
        <template #default="{ row }">
          <el-tag>{{ row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column
        prop="webhook_url"
        :label="$tt('Webhook 地址')"
        min-width="260"
        show-overflow-tooltip
      />
      <el-table-column :label="$tt('签名密钥')" width="110">
        <template #default="{ row }">
          {{ row.secret_configured ? tt("已配置") : tt("未配置") }}
        </template>
      </el-table-column>
      <el-table-column :label="$tt('状态')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">
            {{ row.enabled ? "enabled" : "disabled" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$tt('通知项目组')" min-width="180">
        <template #default="{ row }">
          <el-tag
            v-for="teamId in row.team_ids"
            :key="teamId"
            class="mr-2"
            type="warning"
          >
            {{ teams.find(team => team.id === teamId)?.name ?? teamId }}
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
    :title="editingId === null ? $tt('新建通知渠道') : $tt('修改通知渠道')"
    width="640px"
  >
    <el-form label-position="top" @submit.prevent="submit">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item :label="$tt('类型')" required>
            <el-select v-model="form.type" class="w-full">
              <el-option
                v-for="option in typeOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item :label="$tt('接收角色')">
            <el-select
              v-model="form.role_ids"
              class="w-full"
              multiple
              :placeholder="$tt('选择角色')"
            >
              <el-option
                v-for="role in roles"
                :key="role.id"
                :label="role.name"
                :value="role.id"
              />
            </el-select>
            <div class="text-xs text-gray-500">
              {{ $tt("未选择角色时，仅超级管理员接收通知") }}
            </div>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item :label="$tt('通知项目组')" required>
            <el-checkbox-group
              :model-value="Array.from(selectedTeamIds)"
              @update:model-value="
                (ids: number[]) => (selectedTeamIds = new Set(ids))
              "
            >
              <el-checkbox
                v-for="team in teams"
                :key="team.id"
                :value="team.id"
              >
                {{ team.name }}
              </el-checkbox>
            </el-checkbox-group>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('名称')" required>
            <el-input v-model="form.name" />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item :label="$tt('Webhook 地址')" required>
            <el-input v-model="form.webhook_url" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item
            :label="
              editingId === null
                ? $tt('签名密钥')
                : $tt('签名密钥（留空则不修改）')
            "
          >
            <el-input v-model="form.secret" show-password type="password" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$tt('启用')">
            <el-switch v-model="form.enabled" />
          </el-form-item>
        </el-col>
      </el-row>
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
