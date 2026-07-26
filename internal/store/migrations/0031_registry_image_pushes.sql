CREATE TABLE registry_image_pushes (
    repository TEXT NOT NULL,
    tag TEXT NOT NULL,
    digest TEXT NOT NULL,
    pushed_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(repository, tag)
);

CREATE INDEX registry_image_pushes_digest_idx
    ON registry_image_pushes(repository, digest);
