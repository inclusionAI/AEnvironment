import { expect, test } from "@playwright/test"

test("manages EnvHub metadata through the hosted console", async ({ page }) => {
  const envName = `console-e2e-${Date.now()}`

  await page.goto("http://localhost:18083/console")
  await expect(page.getByRole("heading", { name: "Metadata Console" })).toBeVisible()

  await page.getByLabel("Name").fill(envName)
  await page.getByLabel("Version").fill("1.0.0")
  await page.getByLabel("Description").fill("Created through the hosted console")
  await page.getByLabel("Tags").fill("linux, e2e")
  await page.getByLabel("Code URL").fill(`oss://${envName}`)
  await page
    .getByLabel("Deploy Config")
    .fill(
      JSON.stringify({ cpu: "2", memory: "4Gi", os: "linux", ephemeralStorage: "6Gi" }, null, 2),
    )
  await page.getByRole("button", { name: "Save Meta" }).click()

  await expect(page.getByText(`Saved ${envName}:1.0.0`)).toBeVisible()
  await expect(page.getByRole("button", { name: new RegExp(envName, "i") })).toBeVisible()
  await expect(page.getByLabel("Description")).toHaveValue("Created through the hosted console")

  await page.getByLabel("Tags").fill("linux, e2e, edited")
  await page
    .getByLabel("Deploy Config")
    .fill(
      JSON.stringify({ cpu: "3", memory: "5Gi", os: "linux", ephemeralStorage: "8Gi" }, null, 2),
    )
  await page.getByRole("button", { name: "Save Meta" }).click()

  await expect(page.getByText(`Saved ${envName}:1.0.0`)).toBeVisible()
  await page.reload()
  await expect(page.getByRole("button", { name: new RegExp(envName, "i") })).toBeVisible()
  await expect(page.getByLabel("Tags")).toHaveValue("linux, e2e, edited")
  await expect(page.getByLabel("Deploy Config")).toContainText('"cpu": "3"')
})
