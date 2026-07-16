import { lazy } from 'react'
import { createBrowserRouter } from 'react-router-dom'
import { AppLayout } from '../layouts/AppLayout'
import { AdminLayout } from '../layouts/AdminLayout'
import { RouteErrorPage } from '../components/system/RouteErrorPage'

const HomePage = lazy(() => import('../pages/HomePage').then((module) => ({ default: module.HomePage })))
const ChallengeListPage = lazy(() => import('../pages/ChallengeListPage').then((module) => ({ default: module.ChallengeListPage })))
const ChallengeDetailPage = lazy(() => import('../pages/ChallengeDetailPage').then((module) => ({ default: module.ChallengeDetailPage })))
const CompetitionCenterPage = lazy(() => import('../pages/CompetitionCenterPage').then((module) => ({ default: module.CompetitionCenterPage })))
const CompetitionDetailPage = lazy(() => import('../pages/CompetitionDetailPage').then((module) => ({ default: module.CompetitionDetailPage })))
const CompetitionPlayPage = lazy(() => import('../pages/CompetitionPlayPage').then((module) => ({ default: module.CompetitionPlayPage })))
const LearningPage = lazy(() => import('../pages/LearningPage').then((module) => ({ default: module.LearningPage })))
const LeaderboardPage = lazy(() => import('../pages/LeaderboardPage').then((module) => ({ default: module.LeaderboardPage })))
const TeamsPage = lazy(() => import('../pages/TeamsPage').then((module) => ({ default: module.TeamsPage })))
const TeamDetailPage = lazy(() => import('../pages/TeamDetailPage').then((module) => ({ default: module.TeamDetailPage })))
const WriteupsPage = lazy(() => import('../pages/WriteupsPage').then((module) => ({ default: module.WriteupsPage })))
const WriteupDetailPage = lazy(() => import('../pages/WriteupDetailPage').then((module) => ({ default: module.WriteupDetailPage })))
const ProfilePage = lazy(() => import('../pages/ProfilePage').then((module) => ({ default: module.ProfilePage })))
const AuthPage = lazy(() => import('../pages/AuthPage').then((module) => ({ default: module.AuthPage })))
const AccountActionPage = lazy(() => import('../pages/AccountActionPage').then((module) => ({ default: module.AccountActionPage })))
const AccountSecurityPage = lazy(() => import('../pages/AccountSecurityPage').then((module) => ({ default: module.AccountSecurityPage })))
const TeamInvitationPage = lazy(() => import('../pages/TeamInvitationPage').then((module) => ({ default: module.TeamInvitationPage })))
const NotificationCenterPage = lazy(() => import('../pages/NotificationCenterPage').then((module) => ({ default: module.NotificationCenterPage })))
const WriteupEditorPage = lazy(() => import('../pages/WriteupEditorPage').then((module) => ({ default: module.WriteupEditorPage })))
const TeamManagerPage = lazy(() => import('../pages/TeamManagerPage').then((module) => ({ default: module.TeamManagerPage })))
const ProfileEditPage = lazy(() => import('../pages/ProfileEditPage').then((module) => ({ default: module.ProfileEditPage })))
const UserManagerPage = lazy(() => import('../pages/admin/UserManagerPage').then((module) => ({ default: module.UserManagerPage })))
const ModerationPage = lazy(() => import('../pages/admin/ModerationPage').then((module) => ({ default: module.ModerationPage })))
const NotFoundPage = lazy(() => import('../pages/NotFoundPage').then((module) => ({ default: module.NotFoundPage })))
const AdminSectionPage = lazy(() => import('../pages/AdminSectionPage').then((module) => ({ default: module.AdminSectionPage })))
const AssetCenterPage = lazy(() => import('../pages/admin/AssetCenterPage').then((module) => ({ default: module.AssetCenterPage })))
const AppearanceSlotsPage = lazy(() => import('../pages/admin/AppearanceSlotsPage').then((module) => ({ default: module.AppearanceSlotsPage })))
const BackgroundManagerPage = lazy(() => import('../pages/admin/BackgroundManagerPage').then((module) => ({ default: module.BackgroundManagerPage })))
const AssetAuditPage = lazy(() => import('../pages/admin/AssetAuditPage').then((module) => ({ default: module.AssetAuditPage })))
const PlatformSettingsPage = lazy(() => import('../pages/admin/PlatformSettingsPage').then((module) => ({ default: module.PlatformSettingsPage })))
const ChallengeManagerPage = lazy(() => import('../pages/admin/ChallengeManagerPage').then((module) => ({ default: module.ChallengeManagerPage })))
const CompetitionManagerPage = lazy(() => import('../pages/admin/CompetitionManagerPage').then((module) => ({ default: module.CompetitionManagerPage })))
const InstanceManagerPage = lazy(() => import('../pages/admin/InstanceManagerPage').then((module) => ({ default: module.InstanceManagerPage })))
const RegistryCredentialPage = lazy(() => import('../pages/admin/RegistryCredentialPage').then((module) => ({ default: module.RegistryCredentialPage })))
const LearningManagerPage = lazy(() => import('../pages/admin/LearningManagerPage').then((module) => ({ default: module.LearningManagerPage })))

export const router = createBrowserRouter([
  { path: '/', element: <AppLayout />, errorElement: <RouteErrorPage />, children: [
    { index: true, element: <HomePage /> },
    { path: 'challenges', element: <ChallengeListPage /> },
    { path: 'challenges/:id', element: <ChallengeDetailPage /> },
    { path: 'competitions', element: <CompetitionCenterPage /> },
    { path: 'competitions/:id', element: <CompetitionDetailPage /> },
    { path: 'competitions/:id/play', element: <CompetitionPlayPage /> },
    { path: 'learning', element: <LearningPage /> },
    { path: 'teams', element: <TeamsPage /> },
    { path: 'teams/:id', element: <TeamDetailPage /> },
    { path: 'leaderboard', element: <LeaderboardPage /> },
    { path: 'writeups', element: <WriteupsPage /> },
    { path: 'writeups/:id', element: <WriteupDetailPage /> },
    { path: 'profile', element: <ProfilePage /> },
    { path: 'login', element: <AuthPage mode="login" /> },
    { path: 'register', element: <AuthPage mode="register" /> },
    { path: 'forgot-password', element: <AccountActionPage mode="forgot" /> },
    { path: 'reset-password', element: <AccountActionPage mode="reset" /> },
    { path: 'verify-email', element: <AccountActionPage mode="verify" /> },
    { path: 'confirm-email-change', element: <AccountActionPage mode="change-email" /> },
    { path: 'account/security', element: <AccountSecurityPage /> },
    { path: 'team-invitations/:id', element: <TeamInvitationPage /> },
    { path: 'notifications', element: <NotificationCenterPage /> },
    { path: 'writeups/new', element: <WriteupEditorPage /> },
    { path: 'writeups/:id/edit', element: <WriteupEditorPage /> },
    { path: 'team/manage', element: <TeamManagerPage /> },
    { path: 'profile/edit', element: <ProfileEditPage /> },
    { path: '*', element: <NotFoundPage /> },
  ]},
  { path: '/admin', element: <AdminLayout />, errorElement: <RouteErrorPage />, children: [
    { index: true, element: <AdminSectionPage section="overview" /> },
    { path: 'assets', element: <AssetCenterPage /> },
    { path: 'platform', element: <PlatformSettingsPage /> },
    { path: 'assets/audit', element: <AssetAuditPage /> },
    { path: 'appearance/slots', element: <AppearanceSlotsPage /> },
    { path: 'appearance/backgrounds', element: <BackgroundManagerPage /> },
    { path: 'challenges', element: <ChallengeManagerPage /> },
    { path: 'learning', element: <LearningManagerPage /> },
    { path: 'competitions', element: <CompetitionManagerPage /> },
    { path: 'instances', element: <InstanceManagerPage /> },
    { path: 'registry-credentials', element: <RegistryCredentialPage /> },
    { path: 'users', element: <UserManagerPage /> },
    { path: 'submissions', element: <AdminSectionPage section="submissions" /> },
    { path: 'anti-cheat', element: <ModerationPage kind="anti-cheat" /> },
    { path: 'writeups', element: <ModerationPage kind="writeups" /> },
    { path: 'announcements', element: <AdminSectionPage section="announcements" /> },
    { path: 'settings', element: <AdminSectionPage section="settings" /> },
  ]},
])
