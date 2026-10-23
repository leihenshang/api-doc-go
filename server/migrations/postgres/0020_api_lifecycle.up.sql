-- 0020_api_lifecycle：接口生命周期（RM-04）。
-- status: 1=草稿 2=已发布 3=已废弃；存量接口全部按已发布处理，避免导出突然变空。
-- version: 业务版本号（可空）；deprecated_at: 进入废弃状态的时间（由 service 维护）。
ALTER TABLE "api" ADD COLUMN "status" SMALLINT NOT NULL DEFAULT 2;
ALTER TABLE "api" ADD COLUMN "version" VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "deprecated_at" TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS "idx_api_status" ON "api"("status");
