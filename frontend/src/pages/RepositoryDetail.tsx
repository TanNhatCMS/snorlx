import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, ExternalLink, Workflow, Lock, Globe, Loader2, ChevronDown, ChevronRight, RefreshCw } from 'lucide-react';
import {
  Radar,
  RadarChart,
  PolarGrid,
  PolarAngleAxis,
  PolarRadiusAxis,
  ResponsiveContainer,
  Tooltip,
} from 'recharts';
import { repositoriesApi, workflowsApi } from '../services/api';
import { useTheme } from '../context/ThemeContext';
import { cn } from '../lib/utils';

const SCORE_CATEGORIES = [
  { key: 'security_score', label: 'Bảo mật' },
  { key: 'testing_score', label: 'Kiểm thử' },
  { key: 'cicd_score', label: 'CI/CD' },
  { key: 'documentation_score', label: 'Tài liệu' },
  { key: 'code_quality_score', label: 'Chất lượng code' },
  { key: 'maintenance_score', label: 'Bảo trì' },
  { key: 'community_score', label: 'Cộng đồng' },
] as const;

const CHECK_TO_CATEGORY: Record<string, string> = {
  branch_protection: 'Bảo mật',
  requires_reviews: 'Bảo mật',
  requires_status_checks: 'Bảo mật',
  enforce_admins: 'Bảo mật',
  no_force_pushes: 'Bảo mật',
  dependabot_enabled: 'Bảo mật',
  code_scanning_enabled: 'Bảo mật',
  security_md: 'Bảo mật',
  test_directory: 'Kiểm thử',
  test_config: 'Kiểm thử',
  ci_workflow_tests: 'Kiểm thử',
  has_workflows: 'CI/CD',
  status_checks_required: 'CI/CD',
  readme_exists: 'Tài liệu',
  readme_size: 'Tài liệu',
  license_exists: 'Tài liệu',
  contributing: 'Tài liệu',
  changelog: 'Tài liệu',
  description_set: 'Tài liệu',
  linter_config: 'Chất lượng code',
  editorconfig: 'Chất lượng code',
  type_config: 'Chất lượng code',
  dockerfile: 'Chất lượng code',
  pre_commit: 'Chất lượng code',
  pushed_recently_30: 'Bảo trì',
  pushed_recently_90: 'Bảo trì',
  not_archived: 'Bảo trì',
  topics_set: 'Cộng đồng',
  issue_templates: 'Cộng đồng',
  pr_template: 'Cộng đồng',
  codeowners: 'Cộng đồng',
  code_of_conduct: 'Cộng đồng',
};

const CHECK_META: Record<string, { description: string; remediation: string }> = {
  branch_protection: {
    description: 'Nhánh mặc định có rule bảo vệ để chặn force push và yêu cầu review.',
    remediation: 'Vào Settings → Branches → Thêm branch protection rule cho nhánh mặc định. Bật "Require a pull request before merging" và chặn force push.',
  },
  requires_reviews: {
    description: 'Branch protection yêu cầu ít nhất một review duyệt trước khi merge.',
    remediation: 'Trong branch protection rule, bật "Require approvals" và đặt số review duyệt tối thiểu từ 1 trở lên.',
  },
  requires_status_checks: {
    description: 'Branch protection yêu cầu CI status check phải pass trước khi merge.',
    remediation: 'Trong branch protection rule, bật "Require status checks to pass" và thêm tên workflow hoặc check liên quan.',
  },
  enforce_admins: {
    description: 'Branch protection rule áp dụng cả cho administrator.',
    remediation: 'Trong branch protection rule, bật "Do not allow bypassing the above settings" (áp dụng cho admin).',
  },
  no_force_pushes: {
    description: 'Force push lên nhánh đã bị chặn.',
    remediation: 'Trong branch protection rule, đặt "Allow force pushes" thành "Do not allow force pushes".',
  },
  dependabot_enabled: {
    description: 'Dependabot alerts đã bật để theo dõi dependency có lỗ hổng.',
    remediation: 'Vào Settings → Security → Code security and analysis → Bật "Dependabot alerts".',
  },
  code_scanning_enabled: {
    description: 'Code scanning (vd. CodeQL) được cấu hình để tìm vấn đề bảo mật.',
    remediation: 'Vào Settings → Security → Code security and analysis → Cấu hình Code scanning (vd. CodeQL mặc định).',
  },
  security_md: {
    description: 'File SECURITY.md mô tả cách báo cáo lỗ hổng.',
    remediation: 'Thêm file SECURITY.md ở gốc repo hoặc trong .github/ mô tả chính sách bảo mật và cách báo cáo vấn đề.',
  },
  test_directory: {
    description: 'Repository có thư mục riêng cho test (vd. test/, tests/, spec/).',
    remediation: 'Thêm thư mục test (test, tests, spec, hoặc __tests__) và đặt test vào đó.',
  },
  test_config: {
    description: 'Đã có file cấu hình test framework (vd. jest, pytest, vitest).',
    remediation: 'Thêm file cấu hình cho test framework (vd. jest.config.js, pytest.ini, vitest.config.ts).',
  },
  ci_workflow_tests: {
    description: 'Ít nhất một CI workflow chạy test (tên chứa test, ci, hoặc build).',
    remediation: 'Thêm hoặc đổi tên một workflow trong .github/workflows/ để chạy test và tên gợi ý việc test (vd. "CI" hoặc "Tests").',
  },
  has_workflows: {
    description: 'Repository có ít nhất một GitHub Actions workflow.',
    remediation: 'Thêm một hoặc nhiều file workflow vào .github/workflows/ (vd. ci.yml) để tự động hóa build hoặc test.',
  },
  status_checks_required: {
    description: 'Branch protection yêu cầu status check, đảm bảo CI chạy trước khi merge.',
    remediation: 'Trong branch protection rule, bật "Require status checks" và liệt kê tên workflow hoặc check.',
  },
  readme_exists: {
    description: 'Đã có file README mô tả dự án.',
    remediation: 'Thêm README.md ở gốc repository với tổng quan dự án, cách cài đặt và sử dụng.',
  },
  readme_size: {
    description: 'README có đủ nội dung (vd. ít nhất ~500 byte).',
    remediation: 'Mở rộng README.md với mô tả rõ ràng, các bước cài đặt và ví dụ sử dụng.',
  },
  license_exists: {
    description: 'Đã có file LICENSE với mã SPDX hợp lệ.',
    remediation: 'Thêm LICENSE ở gốc (vd. MIT, Apache-2.0) để điều khoản tái sử dụng rõ ràng.',
  },
  contributing: {
    description: 'File CONTRIBUTING giải thích cách đóng góp.',
    remediation: 'Thêm CONTRIBUTING.md hoặc CONTRIBUTING ở gốc với hướng dẫn và quy trình đóng góp.',
  },
  changelog: {
    description: 'File CHANGELOG ghi lại lịch sử phiên bản.',
    remediation: 'Thêm CHANGELOG.md ở gốc và cập nhật các thay đổi đáng chú ý theo từng release.',
  },
  description_set: {
    description: 'Repository đã đặt mô tả ngắn trên GitHub.',
    remediation: 'Sửa mô tả repository trên trang chính của repo GitHub (phía dưới tên).',
  },
  linter_config: {
    description: 'Đã có cấu hình linter/formatter (vd. ESLint, Prettier, Rubocop).',
    remediation: 'Thêm file cấu hình cho ngôn ngữ của bạn (vd. .eslintrc.json, .prettierrc, .rubocop.yml).',
  },
  editorconfig: {
    description: 'File .editorconfig ép thống nhất cấu hình editor.',
    remediation: 'Thêm .editorconfig ở gốc để định nghĩa indentation, charset và line ending.',
  },
  type_config: {
    description: 'Đã có cấu hình kiểm tra kiểu (vd. tsconfig cho TypeScript).',
    remediation: 'Thêm tsconfig.json (hoặc tương đương) nếu dùng TypeScript hoặc ngôn ngữ có kiểu khác.',
  },
  dockerfile: {
    description: 'Đã có Dockerfile để build hoặc chạy trong container.',
    remediation: 'Thêm Dockerfile ở gốc (hoặc thư mục riêng) để build container.',
  },
  pre_commit: {
    description: 'Pre-commit hook đã được cấu hình (vd. .pre-commit-config.yaml).',
    remediation: 'Thêm .pre-commit-config.yaml và cài pre-commit để chạy kiểm tra trước mỗi commit.',
  },
  pushed_recently_30: {
    description: 'Repository có ít nhất một lần push trong 30 ngày qua.',
    remediation: 'Commit và push lên nhánh mặc định để thể hiện hoạt động gần đây.',
  },
  pushed_recently_90: {
    description: 'Repository có ít nhất một lần push trong 90 ngày qua.',
    remediation: 'Commit và push lên nhánh mặc định để thể hiện hoạt động gần đây.',
  },
  not_archived: {
    description: 'Repository không bị archive và vẫn được bảo trì.',
    remediation: 'Unarchive repository trong Settings → Danger Zone nếu cần hoạt động lại.',
  },
  topics_set: {
    description: 'Repository có một hoặc nhiều topics (nhãn) trên GitHub.',
    remediation: 'Thêm topics trên trang chính repo (nhấn bánh răng cạnh "About") để dễ được tìm thấy.',
  },
  issue_templates: {
    description: 'Issue template được cấu hình cho báo lỗi hoặc đề xuất tính năng có cấu trúc.',
    remediation: 'Thêm issue template vào .github/ISSUE_TEMPLATE/ hoặc dùng file .github/ISSUE_TEMPLATE.md đơn.',
  },
  pr_template: {
    description: 'Pull request template đã được cấu hình.',
    remediation: 'Thêm .github/PULL_REQUEST_TEMPLATE.md hoặc file trong .github/PULL_REQUEST_TEMPLATE/ để hướng dẫn viết PR.',
  },
  codeowners: {
    description: 'File CODEOWNERS định nghĩa owner cho các đường dẫn trong repo.',
    remediation: 'Thêm CODEOWNERS ở gốc hoặc .github/ để gán owner (vd. * @team/backend).',
  },
  code_of_conduct: {
    description: 'File code of conduct đặt kỳ vọng hành vi cho cộng đồng.',
    remediation: 'Add CODE_OF_CONDUCT.md in the root or dùng template của GitHub từ Insights → Community.',
  },
};

function CheckBox({ check, pass }: { check: string; pass: boolean }) {
  const meta = CHECK_META[check];
  const label = check.replace(/_/g, ' ');
  return (
    <div className="relative group">
      <div
        className={cn(
          'min-h-[2.25rem] rounded-md px-2 py-1.5 text-xs font-medium flex items-center justify-center text-center break-words border-2 cursor-help',
          pass
            ? 'bg-green-500/15 border-green-500/70 text-green-700 dark:bg-green-500/20 dark:border-green-400/80 dark:text-green-300'
            : 'bg-red-500/15 border-red-500/70 text-red-700 dark:bg-red-500/20 dark:border-red-400/80 dark:text-red-300',
        )}
      >
        <span className="truncate">{label}</span>
      </div>
      {(meta || label) && (
        <div
          className="pointer-events-none invisible absolute left-0 z-50 mt-1.5 w-[min(18rem,calc(100vw-2rem))] rounded-lg border border-gray-200 bg-white px-3 py-2 text-left opacity-0 shadow-lg transition-opacity group-hover:visible group-hover:opacity-100 sm:left-1/2 sm:-translate-x-1/2 dark:border-gray-600 dark:bg-gray-800"
          role="tooltip"
        >
          <p className="text-xs font-semibold text-gray-900 dark:text-gray-100 mb-1">{label}</p>
          {meta?.description && (
            <p className="text-xs text-gray-600 dark:text-gray-400 mb-1.5">{meta.description}</p>
          )}
          {!pass && meta?.remediation && (
            <p className="text-xs text-amber-700 dark:text-amber-300 border-t border-gray-200 dark:border-gray-600 pt-1.5 mt-1.5">
              <span className="font-medium">Cách khắc phục:</span> {meta.remediation}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

export function RepositoryDetail() {
  const { id } = useParams<{ id: string }>();
  const { isDark } = useTheme();
  const [checksExpanded, setChecksExpanded] = useState(false);

  const { data: repo, isLoading } = useQuery({
    queryKey: ['repositories', id],
    queryFn: () => repositoriesApi.get(Number(id)),
    enabled: !!id,
  });

  const { data: workflows } = useQuery({
    queryKey: ['workflows', { repo_id: id }],
    queryFn: () => workflowsApi.list(Number(id)),
    enabled: !!id,
  });

  const queryClient = useQueryClient();
  const { data: score } = useQuery({
    queryKey: ['repositories', id, 'score'],
    queryFn: () => repositoriesApi.getScore(Number(id)),
    enabled: !!id,
  });

  const refreshGradeMutation = useMutation({
    mutationFn: () => repositoriesApi.syncRepo(Number(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repositories', id, 'score'] });
      queryClient.invalidateQueries({ queryKey: ['repositories', id] });
      queryClient.invalidateQueries({ queryKey: ['workflows', { repo_id: id }] });
    },
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Loader2 className="w-8 h-8 animate-spin text-primary-500" />
      </div>
    );
  }

  if (!repo) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500 dark:text-gray-400">Không tìm thấy repository</p>
      </div>
    );
  }

  return (
    <div className="min-w-0 space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 items-start gap-3 sm:gap-4">
          <Link
            to="/repositories"
            className="shrink-0 rounded-lg p-2 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
            aria-label="Quay lại danh sách repository"
          >
            <ArrowLeft className="h-5 w-5 text-gray-500" />
          </Link>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="break-words text-xl font-bold text-gray-900 sm:text-2xl dark:text-gray-100">{repo.name}</h1>
              {repo.is_private ? (
                <Lock className="w-4 h-4 text-gray-400" />
              ) : (
                <Globe className="w-4 h-4 text-gray-400" />
              )}
            </div>
            <p className="break-all text-gray-500 dark:text-gray-400">{repo.full_name}</p>
          </div>
        </div>
        <a
          href={repo.html_url}
          target="_blank"
          rel="noopener noreferrer"
          className="btn-secondary inline-flex w-fit shrink-0 items-center gap-2"
        >
          <ExternalLink className="w-4 h-4" />
          Xem trên GitHub
        </a>
      </div>

      {/* Description */}
      {repo.description && (
        <div className="card p-4">
          <p className="text-gray-600 dark:text-gray-300">{repo.description}</p>
        </div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="card p-4">
          <p className="text-sm text-gray-500 dark:text-gray-400">Nhánh mặc định</p>
          <p className="text-xl font-bold text-gray-900 dark:text-gray-100">{repo.default_branch}</p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-gray-500 dark:text-gray-400">Workflows</p>
          <p className="text-xl font-bold text-gray-900 dark:text-gray-100">{workflows?.length || 0}</p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-gray-500 dark:text-gray-400">Hiển thị</p>
          <p className="text-xl font-bold text-gray-900 dark:text-gray-100">
            {repo.is_private ? 'Riêng tư' : 'Công khai'}
          </p>
        </div>
      </div>

      {/* Repository Score */}
      {(score || repo) && (
        <div className="card">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-gray-700">
            <div>
              <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Điểm repository</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
                {score
                  ? `Quét lần cuối: ${new Date(score.scanned_at).toLocaleString()}`
                  : 'Chưa có điểm. Sync để chấm điểm repository này.'}
              </p>
            </div>
            <button
              type="button"
              onClick={() => refreshGradeMutation.mutate()}
              disabled={refreshGradeMutation.isPending}
              className="btn btn-secondary flex items-center gap-2"
              title="Kiểm tra lại trên GitHub và cập nhật điểm"
            >
              <RefreshCw className={cn('w-4 h-4', refreshGradeMutation.isPending && 'animate-spin')} />
              {refreshGradeMutation.isPending ? 'Đang kiểm tra…' : 'Làm mới điểm'}
            </button>
          </div>
          <div className="space-y-6 p-4 sm:p-6">
            {!score ? (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Nhấn &quot;Làm mới điểm&quot; để kiểm tra repository này trên GitHub và tính điểm.
              </p>
            ) : (
              <>
            <div className="flex flex-wrap items-center gap-4">
              <span className="text-3xl font-bold text-gray-900 dark:text-gray-100">
                {Math.round(score.overall_score)}%
              </span>
              <span
                className={
                  score.tier === 'gold'
                    ? 'badge badge-gold'
                    : score.tier === 'silver'
                      ? 'badge badge-silver'
                      : 'badge badge-bronze'
                }
              >
                {score.tier === 'none' ? 'bronze' : score.tier}
              </span>
            </div>
            <div className="h-[220px] min-w-0 sm:h-[280px]">
              <ResponsiveContainer width="100%" height="100%">
                <RadarChart
                  margin={{ top: 8, right: 24, bottom: 8, left: 24 }}
                  data={SCORE_CATEGORIES.map(({ key, label }) => ({
                    subject: label,
                    score:
                      key === 'security_score'
                        ? score.security_score
                        : key === 'testing_score'
                          ? score.testing_score
                          : key === 'cicd_score'
                            ? score.cicd_score
                            : key === 'documentation_score'
                              ? score.documentation_score
                              : key === 'code_quality_score'
                                ? score.code_quality_score
                                : key === 'maintenance_score'
                                  ? score.maintenance_score
                                  : score.community_score,
                    fullMark: 100,
                  }))}
                >
                  <PolarGrid stroke={isDark ? '#374151' : '#d1d5db'} />
                  <PolarAngleAxis
                    dataKey="subject"
                    tick={{ fill: isDark ? '#9ca3af' : '#6b7280', fontSize: 11 }}
                  />
                  <PolarRadiusAxis
                    angle={90}
                    domain={[0, 100]}
                    tick={{ fill: isDark ? '#9ca3af' : '#6b7280', fontSize: 10 }}
                  />
                  <Radar
                    name="Điểm"
                    dataKey="score"
                    stroke={isDark ? '#34cb6f' : '#2db865'}
                    fill={isDark ? '#34cb6f' : '#2db865'}
                    fillOpacity={0.3}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: isDark ? '#1f2937' : '#fff',
                      border: `1px solid ${isDark ? '#374151' : '#e5e7eb'}`,
                      borderRadius: '8px',
                    }}
                    labelStyle={{ color: isDark ? '#e5e7eb' : '#111827' }}
                    formatter={(value: unknown) => [typeof value === 'number' ? `${value.toFixed(0)}%` : String(value), 'Điểm'] as [React.ReactNode, string]}
                  />
                </RadarChart>
              </ResponsiveContainer>
            </div>
            {score.check_results && Object.keys(score.check_results).length > 0 && (
              <div>
                <button
                  type="button"
                  onClick={() => setChecksExpanded((e) => !e)}
                  className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-primary-600 dark:hover:text-primary-400"
                >
                  {checksExpanded ? (
                    <ChevronDown className="w-4 h-4" />
                  ) : (
                    <ChevronRight className="w-4 h-4" />
                  )}
                  Chi tiết kiểm tra
                </button>
                {checksExpanded && (() => {
                  const order = ['Bảo mật', 'Kiểm thử', 'CI/CD', 'Tài liệu', 'Chất lượng code', 'Bảo trì', 'Cộng đồng'];
                  const byCategory = Object.entries(score.check_results).reduce<Record<string, [string, boolean][]>>((acc, [check, pass]) => {
                    const cat = CHECK_TO_CATEGORY[check] ?? 'Khác';
                    if (!acc[cat]) acc[cat] = [];
                    acc[cat].push([check, pass]);
                    return acc;
                  }, {});
                  return (
                    <div className="mt-3 space-y-4">
                      {order.filter((cat) => byCategory[cat]?.length).map((category) => (
                        <div key={category}>
                          <p className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-2">
                            {category}
                          </p>
                          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                            {byCategory[category].map(([check, pass]) => (
                              <CheckBox key={check} check={check} pass={pass} />
                            ))}
                          </div>
                        </div>
                      ))}
                      {byCategory['Khác']?.length > 0 && (
                        <div>
                          <p className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-2">
                            Khác
                          </p>
                          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                            {byCategory['Khác'].map(([check, pass]) => (
                              <CheckBox key={check} check={check} pass={pass} />
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })()}
              </div>
            )}
              </>
            )}
          </div>
        </div>
      )}

      {/* Workflows */}
      <div className="card">
        <div className="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-gray-700">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Workflows</h2>
        </div>
        <div className="divide-y divide-gray-100 dark:divide-gray-700">
          {workflows && workflows.length > 0 ? (
            workflows.map((workflow) => (
              <Link
                key={workflow.id}
                to={`/workflows/${workflow.id}`}
                className="flex flex-col gap-2 px-4 py-4 transition-colors hover:bg-gray-50 sm:flex-row sm:items-center sm:justify-between sm:px-6 dark:hover:bg-gray-800/50"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <div className="shrink-0 rounded-lg bg-primary-100 p-2 dark:bg-primary-900/30">
                    <Workflow className="h-4 w-4 text-primary-600 dark:text-primary-400" />
                  </div>
                  <div className="min-w-0">
                    <p className="break-words font-medium text-gray-900 dark:text-gray-100">{workflow.name}</p>
                    <p className="break-all text-sm text-gray-500 dark:text-gray-400">{workflow.path}</p>
                  </div>
                </div>
                <span className={`badge ${workflow.state === 'active' ? 'badge-success' : 'badge-neutral'}`}>
                  {workflow.state}
                </span>
              </Link>
            ))
          ) : (
            <div className="px-6 py-8 text-center">
              <Workflow className="w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-3" />
              <p className="text-gray-500 dark:text-gray-400">Không tìm thấy workflow nào</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

