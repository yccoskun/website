import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { Layout } from "@/components/layout";
import { ThemeProvider } from "@/components/theme-provider";
import { AdminLayout } from "@/pages/admin/layout";
import { AdminPostEditorPage } from "@/pages/admin/post-editor";
import { AdminPostsPage } from "@/pages/admin/posts";
import { RequireAdmin } from "@/pages/admin/require-admin";
import { AdminResumePage } from "@/pages/admin/resume";
import { AdminSessionProvider } from "@/pages/admin/session";
import { BlogPage } from "@/pages/blog";
import { HomePage } from "@/pages/home";
import { NotFoundPage } from "@/pages/not-found";
import { PostPage } from "@/pages/post";
import { ResumePage } from "@/pages/resume";

import "@/index.css";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root element");

createRoot(root).render(
  <StrictMode>
    <ThemeProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/blog" element={<BlogPage />} />
            <Route path="/blog/:slug" element={<PostPage />} />
            <Route path="/resume" element={<ResumePage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
          <Route
            path="/admin"
            element={
              <AdminSessionProvider>
                <RequireAdmin>
                  <AdminLayout />
                </RequireAdmin>
              </AdminSessionProvider>
            }
          >
            <Route index element={<Navigate to="posts" replace />} />
            <Route path="posts" element={<AdminPostsPage />} />
            <Route path="posts/new" element={<AdminPostEditorPage />} />
            <Route path="posts/:id" element={<AdminPostEditorPage />} />
            <Route path="resume" element={<AdminResumePage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ThemeProvider>
  </StrictMode>,
);
