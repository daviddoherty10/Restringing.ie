# Verified booking reviews

After a racket is collected, the customer can open the order and save a rating of one to five stars and an optional comment of up to 500 characters. The customer can edit or remove the review from the same order. Each completed booking can have one review. Payment method has no bearing on review eligibility. Business members cannot review their own business.

The public profile shows the average, count and ten most recent eligible reviews only while at least five distinct customers have eligible reviews. Star-only reviews count. Repeat bookings can contribute to the rating, but repeat reviewers count only once toward the publication threshold. If a review is withdrawn, hidden or becomes ineligible and fewer than five customers remain, the API returns neither ratings nor comments. Reviews do not affect search ordering.

Business members can report visible reviews from **Stringer dashboard → Settings → Profile**. A report does not change the public rating. Platform administrators can keep or hide a reported review under **Admin → Reviews**. Editing or resubmitting a hidden review keeps it hidden. Hidden ratings and comments are excluded from the public profile. Moderation decisions are recorded in the compliance audit. Review data is included in the customer’s privacy export and removed on account closure. Use the [private complaints workflow](compliance-implementation.md) for booking disputes or follow-up.

The feature uses migration 22 and the matching API and frontend. Before a live rollout, agree who will handle review reports and how review data and moderation decisions will be handled in the privacy and retention process.
