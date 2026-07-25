import PageHeader from "./components/PageHeader";

/** Thin wrapper so existing pages pick up shared PageHeader styling. */
export default function PageHead({ title, blurb }: { title: string; blurb: string }) {
  return <PageHeader title={title} subtitle={blurb} />;
}
